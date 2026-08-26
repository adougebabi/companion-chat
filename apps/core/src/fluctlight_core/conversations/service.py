"""Conversation persistence and turn application boundary."""

from __future__ import annotations

from collections.abc import AsyncIterator, Callable, Iterable
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import func, insert, select, text, update

from fluctlight_core.platform.persistence import UnitOfWork, UnitOfWorkFactory

from . import schema
from .contracts import (
    Conversation,
    ConversationAuthorizationError,
    ConversationConflictError,
    ConversationNotFoundError,
    ConversationPage,
    ConversationProviderError,
    ConversationTurn,
    Message,
    MessageDraft,
    MessageKind,
    Participant,
    ParticipantRole,
    ParticipantStatus,
    TurnResponder,
    TurnResult,
    TurnStreamEvent,
)


class ConversationService:
    """Own ordered conversation rows; semantic cognition stays behind a port."""

    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        responder: TurnResponder | None = None,
        *,
        clock: Callable[[], datetime] | None = None,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._responder = responder
        self._clock = clock or (lambda: datetime.now(UTC))

    @asynccontextmanager
    async def _transaction(
        self, tx: UnitOfWork | None, command_id: str
    ) -> AsyncIterator[UnitOfWork]:
        if tx is not None:
            yield tx
            return
        async with self._unit_of_work.begin(command_id=command_id) as owned:
            yield owned
            await owned.commit()

    async def create(
        self,
        *,
        actor_id: str,
        participant_actor_ids: Iterable[str] = (),
        title: str | None = None,
        tx: UnitOfWork | None = None,
    ) -> ConversationPage:
        now = self._clock()
        conversation_id = f"conversation_{uuid4().hex}"
        participant_ids = tuple(dict.fromkeys((actor_id, *participant_actor_ids)))
        if len(participant_ids) < 2:
            raise ConversationConflictError(
                "a conversation requires an explicit Fluctlight participant"
            )
        async with self._transaction(tx, f"conversation-create:{conversation_id}") as transaction:
            await transaction.session.execute(
                insert(schema.conversations).values(
                    id=conversation_id,
                    created_by_actor_id=actor_id,
                    title=title,
                    revision=0,
                    created_at=now,
                    updated_at=now,
                )
            )
            await transaction.session.execute(
                insert(schema.conversation_heads).values(
                    conversation_id=conversation_id, next_sequence=1
                )
            )
            for index, participant_id in enumerate(participant_ids):
                await transaction.session.execute(
                    insert(schema.participants).values(
                        conversation_id=conversation_id,
                        actor_id=participant_id,
                        role=ParticipantRole.OWNER.value
                        if index == 0
                        else ParticipantRole.MEMBER.value,
                        status=ParticipantStatus.ACTIVE.value,
                        joined_at=now,
                    )
                )
                await transaction.session.execute(
                    insert(schema.read_positions).values(
                        conversation_id=conversation_id,
                        actor_id=participant_id,
                        last_read_sequence=0,
                        last_delivered_sequence=0,
                        updated_at=now,
                    )
                )
        return ConversationPage(
            conversation=Conversation(
                conversation_id, actor_id, title=title, created_at=now, updated_at=now
            ),
            participants=tuple(
                Participant(
                    conversation_id,
                    participant_id,
                    role=ParticipantRole.OWNER if index == 0 else ParticipantRole.MEMBER,
                    joined_at=now,
                )
                for index, participant_id in enumerate(participant_ids)
            ),
            messages=(),
        )

    async def get_or_create_direct(
        self, *, owner_actor_id: str, fluctlight_actor_id: str
    ) -> ConversationPage:
        """Return the one Owner-to-Fluctlight conversation for product entry.

        PostgreSQL advisory locking serializes first-open requests for the pair.
        The durable unique mapping is the authority after the transaction commits.
        """

        if owner_actor_id == fluctlight_actor_id:
            raise ConversationConflictError("a direct Fluctlight conversation requires two actors")
        lock_key = f"fluctlight-direct:{owner_actor_id}:{fluctlight_actor_id}"
        created_page: ConversationPage | None = None
        existing_conversation_id: str | None = None
        async with self._unit_of_work.begin(
            command_id=f"conversation-direct:{owner_actor_id}:{fluctlight_actor_id}"
        ) as tx:
            await tx.session.execute(
                text("SELECT pg_advisory_xact_lock(hashtext(:lock_key))"),
                {"lock_key": lock_key},
            )
            existing_conversation_id = await tx.session.scalar(
                select(schema.direct_conversations.c.conversation_id).where(
                    schema.direct_conversations.c.owner_actor_id == owner_actor_id,
                    schema.direct_conversations.c.fluctlight_actor_id == fluctlight_actor_id,
                )
            )
            if existing_conversation_id is None:
                created_page = await self.create(
                    actor_id=owner_actor_id,
                    participant_actor_ids=(fluctlight_actor_id,),
                    title=None,
                    tx=tx,
                )
                await tx.session.execute(
                    insert(schema.direct_conversations).values(
                        owner_actor_id=owner_actor_id,
                        fluctlight_actor_id=fluctlight_actor_id,
                        conversation_id=created_page.conversation.id,
                        created_at=self._clock(),
                    )
                )
            await tx.commit()
        if created_page is not None:
            return created_page
        if existing_conversation_id is None:
            raise ConversationConflictError("direct conversation was not created")
        return await self.history(existing_conversation_id, actor_id=owner_actor_id)

    async def direct_unread_counts(
        self, *, owner_actor_id: str, fluctlight_actor_ids: tuple[str, ...]
    ) -> dict[str, int]:
        """Project persisted Owner read positions for the instance directory."""

        if not fluctlight_actor_ids:
            return {}
        async with self._unit_of_work.begin(command_id=f"direct-unread:{owner_actor_id}") as tx:
            rows = (
                await tx.session.execute(
                    select(
                        schema.direct_conversations.c.fluctlight_actor_id,
                        func.count(schema.messages.c.id).label("unread_count"),
                    )
                    .select_from(schema.direct_conversations)
                    .join(
                        schema.read_positions,
                        (schema.read_positions.c.conversation_id
                         == schema.direct_conversations.c.conversation_id)
                        & (schema.read_positions.c.actor_id == owner_actor_id),
                    )
                    .outerjoin(
                        schema.messages,
                        (schema.messages.c.conversation_id
                         == schema.direct_conversations.c.conversation_id)
                        & (schema.messages.c.author_actor_id
                           == schema.direct_conversations.c.fluctlight_actor_id)
                        & (schema.messages.c.sequence
                           > schema.read_positions.c.last_read_sequence),
                    )
                    .where(
                        schema.direct_conversations.c.owner_actor_id == owner_actor_id,
                        schema.direct_conversations.c.fluctlight_actor_id.in_(fluctlight_actor_ids),
                    )
                    .group_by(schema.direct_conversations.c.fluctlight_actor_id)
                )
            ).mappings().all()
        counts = {str(row["fluctlight_actor_id"]): int(row["unread_count"]) for row in rows}
        return {
            fluctlight_id: counts.get(fluctlight_id, 0)
            for fluctlight_id in fluctlight_actor_ids
        }

    async def history(
        self,
        conversation_id: str,
        *,
        actor_id: str,
        before_sequence: int | None = None,
        limit: int = 50,
    ) -> ConversationPage:
        bounded_limit = min(max(limit, 1), 200)
        async with self._unit_of_work.begin(
            command_id=f"conversation-history:{conversation_id}"
        ) as tx:
            conversation = await self._conversation(tx, conversation_id)
            participants = await self._participants(tx, conversation_id)
            self._require_member(participants, actor_id)
            statement = select(schema.messages).where(
                schema.messages.c.conversation_id == conversation_id
            )
            if before_sequence is not None:
                if before_sequence < 1:
                    raise ValueError("before_sequence must be positive")
                statement = statement.where(schema.messages.c.sequence < before_sequence)
            statement = statement.order_by(schema.messages.c.sequence.desc()).limit(
                bounded_limit + 1
            )
            rows = (await tx.session.execute(statement)).mappings().all()
        has_more = len(rows) > bounded_limit
        selected = tuple(self._message_from_row(row) for row in reversed(rows[:bounded_limit]))
        next_before = selected[0].sequence if has_more and selected else None
        return ConversationPage(conversation, tuple(participants), selected, next_before)

    async def append_message(
        self,
        conversation_id: str,
        draft: MessageDraft,
        *,
        actor_id: str,
        tx: UnitOfWork | None = None,
    ) -> Message:
        now = self._clock()
        async with self._transaction(
            tx, f"conversation-message:{draft.idempotency_key}"
        ) as transaction:
            participants = await self._participants(transaction, conversation_id)
            self._require_member(participants, actor_id)
            if draft.author_actor_id != actor_id:
                raise ConversationAuthorizationError("message author must match the resolved actor")
            existing = (
                (
                    await transaction.session.execute(
                        select(schema.messages).where(
                            schema.messages.c.conversation_id == conversation_id,
                            schema.messages.c.idempotency_key == draft.idempotency_key,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                if (
                    existing["author_actor_id"] != draft.author_actor_id
                    or existing["text"] != draft.text
                ):
                    raise ConversationConflictError("message idempotency key was reused")
                return self._message_from_row(existing)
            head = (
                (
                    await transaction.session.execute(
                        select(schema.conversation_heads)
                        .where(schema.conversation_heads.c.conversation_id == conversation_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if head is None:
                raise ConversationNotFoundError(conversation_id)
            sequence = int(head["next_sequence"])
            message_id = f"message_{uuid4().hex}"
            await transaction.session.execute(
                insert(schema.messages).values(
                    id=message_id,
                    conversation_id=conversation_id,
                    sequence=sequence,
                    author_actor_id=draft.author_actor_id,
                    kind=draft.kind.value,
                    text=draft.text,
                    attachment_refs=list(draft.attachment_refs),
                    idempotency_key=draft.idempotency_key,
                    created_at=now,
                )
            )
            await transaction.session.execute(
                update(schema.conversation_heads)
                .where(schema.conversation_heads.c.conversation_id == conversation_id)
                .values(next_sequence=sequence + 1)
            )
            await transaction.session.execute(
                update(schema.conversations)
                .where(schema.conversations.c.id == conversation_id)
                .values(revision=schema.conversations.c.revision + 1, updated_at=now)
            )
            return Message(
                message_id,
                conversation_id,
                sequence,
                draft.author_actor_id,
                draft.text,
                draft.kind,
                draft.attachment_refs,
                now,
                draft.idempotency_key,
            )

    async def mark_read(
        self,
        conversation_id: str,
        *,
        actor_id: str,
        read_sequence: int,
        delivered_sequence: int | None = None,
    ) -> None:
        if read_sequence < 0 or (
            delivered_sequence is not None and delivered_sequence < read_sequence
        ):
            raise ValueError("read/delivery positions are invalid")
        async with self._unit_of_work.begin(
            command_id=f"conversation-read:{conversation_id}:{actor_id}"
        ) as tx:
            participants = await self._participants(tx, conversation_id)
            self._require_member(participants, actor_id)
            current = (
                (
                    await tx.session.execute(
                        select(schema.read_positions)
                        .where(
                            schema.read_positions.c.conversation_id == conversation_id,
                            schema.read_positions.c.actor_id == actor_id,
                        )
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            next_read = max(read_sequence, int(current["last_read_sequence"]) if current else 0)
            next_delivered = max(
                delivered_sequence or 0,
                int(current["last_delivered_sequence"]) if current else 0,
                next_read,
            )
            if current is None:
                await tx.session.execute(
                    insert(schema.read_positions).values(
                        conversation_id=conversation_id,
                        actor_id=actor_id,
                        last_read_sequence=next_read,
                        last_delivered_sequence=next_delivered,
                        updated_at=self._clock(),
                    )
                )
            else:
                await tx.session.execute(
                    update(schema.read_positions)
                    .where(
                        schema.read_positions.c.conversation_id == conversation_id,
                        schema.read_positions.c.actor_id == actor_id,
                    )
                    .values(
                        last_read_sequence=next_read,
                        last_delivered_sequence=next_delivered,
                        updated_at=self._clock(),
                    )
                )
            await tx.commit()

    async def accept_turn(self, turn: ConversationTurn) -> TurnResult:
        await self._require_turn_target(turn)
        user_message = await self.append_message(
            turn.conversation_id,
            MessageDraft(
                author_actor_id=turn.actor_id,
                text=turn.text,
                kind=MessageKind.USER,
                attachment_refs=turn.attachment_refs,
                idempotency_key=f"{turn.idempotency_key}:user",
            ),
            actor_id=turn.actor_id,
        )
        if self._responder is None:
            raise ConversationProviderError("conversation responder is unavailable")
        page = await self.history(turn.conversation_id, actor_id=turn.actor_id, limit=200)
        response = await self._responder.respond(turn, page.messages)
        assistant_messages: list[Message] = []
        for index, draft in enumerate(response.messages):
            assistant_messages.append(
                await self.append_message(
                    turn.conversation_id,
                    draft,
                    actor_id=draft.author_actor_id,
                )
            )
        events = response.events or tuple(
            {"type": "message", "message_id": message.id, "text": message.text}
            for message in assistant_messages
        )
        return TurnResult(turn, user_message, tuple(assistant_messages), events)

    async def stream_accept_turn(self, turn: ConversationTurn) -> AsyncIterator[TurnStreamEvent]:
        await self._require_turn_target(turn)
        user_message = await self.append_message(
            turn.conversation_id,
            MessageDraft(
                author_actor_id=turn.actor_id,
                text=turn.text,
                kind=MessageKind.USER,
                attachment_refs=turn.attachment_refs,
                idempotency_key=f"{turn.idempotency_key}:user",
            ),
            actor_id=turn.actor_id,
        )
        if self._responder is None:
            raise ConversationProviderError("conversation responder is unavailable")
        yield TurnStreamEvent("action_result", message=user_message)
        page = await self.history(turn.conversation_id, actor_id=turn.actor_id, limit=200)
        stream_respond = getattr(self._responder, "stream_respond", None)
        assistant_text = ""
        if stream_respond is not None:
            async for chunk in stream_respond(turn, page.messages):
                assistant_text += chunk
                yield TurnStreamEvent("token", text=chunk)
        else:
            response = await self._responder.respond(turn, page.messages)
            for draft in response.messages:
                if draft.kind is not MessageKind.MEDIA_REFERENCE:
                    for start in range(0, len(draft.text), 64):
                        chunk = draft.text[start : start + 64]
                        assistant_text += chunk
                        yield TurnStreamEvent("token", text=chunk)
                else:
                    assistant_text = draft.text
        assistant_messages: list[Message] = []
        if assistant_text:
            assistant_messages.append(
                await self.append_message(
                    turn.conversation_id,
                    MessageDraft(
                        author_actor_id=turn.fluctlight_id or turn.actor_id,
                        text=assistant_text,
                        kind=MessageKind.ASSISTANT,
                        idempotency_key=f"{turn.idempotency_key}:assistant",
                    ),
                    actor_id=turn.fluctlight_id or turn.actor_id,
                )
            )
        yield TurnStreamEvent(
            "completed", message_ids=tuple(message.id for message in assistant_messages)
        )

    async def _conversation(self, tx: UnitOfWork, conversation_id: str) -> Conversation:
        row = (
            (
                await tx.session.execute(
                    select(schema.conversations).where(schema.conversations.c.id == conversation_id)
                )
            )
            .mappings()
            .one_or_none()
        )
        if row is None:
            raise ConversationNotFoundError(conversation_id)
        return Conversation(
            row["id"],
            row["created_by_actor_id"],
            row["title"],
            int(row["revision"]),
            row["created_at"],
            row["updated_at"],
        )

    async def _require_turn_target(self, turn: ConversationTurn) -> None:
        if not turn.fluctlight_id:
            raise ConversationAuthorizationError("turn requires an explicit Fluctlight participant")
        async with self._unit_of_work.begin(
            command_id=f"conversation-turn-target:{turn.conversation_id}:{turn.turn_id}"
        ) as tx:
            participants = await self._participants(tx, turn.conversation_id)
            self._require_member(participants, turn.actor_id)
            self._require_member(participants, turn.fluctlight_id)

    async def _participants(self, tx: UnitOfWork, conversation_id: str) -> list[Participant]:
        rows = (
            (
                await tx.session.execute(
                    select(schema.participants).where(
                        schema.participants.c.conversation_id == conversation_id
                    )
                )
            )
            .mappings()
            .all()
        )
        return [
            Participant(
                row["conversation_id"],
                row["actor_id"],
                row["role"],
                row["status"],
                row["joined_at"],
                row["left_at"],
            )
            for row in rows
        ]

    @staticmethod
    def _require_member(participants: Iterable[Participant], actor_id: str) -> None:
        if not any(
            item.actor_id == actor_id and item.status is ParticipantStatus.ACTIVE
            for item in participants
        ):
            raise ConversationAuthorizationError("actor is not an active conversation participant")

    @staticmethod
    def _message_from_row(row: Any) -> Message:
        return Message(
            row["id"],
            row["conversation_id"],
            int(row["sequence"]),
            row["author_actor_id"],
            row["text"],
            MessageKind(row["kind"]),
            tuple(row["attachment_refs"] or ()),
            row["created_at"],
            row["idempotency_key"],
        )
