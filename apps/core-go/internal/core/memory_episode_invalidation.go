package core

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// A corrected or forgotten Memory can no longer lend its old conclusion to
// a sourced conversation episode. We retire only summaries whose raw message
// window contains an evidence message for that Memory. Raw messages remain
// intact and available through the existing history reader.
func invalidateEpisodeSummariesForMemoryTx(ctx context.Context, tx pgx.Tx, command PreparedMemoryMutation, row memoryAuthorityRow) error {
	if row.Revision == 0 || (command.Operation != MemoryRevise && command.Operation != MemoryMerge && command.Operation != MemorySupersede && command.Operation != MemoryDeprecate && command.Operation != MemoryForget && command.Operation != MemoryRollback) {
		return nil
	}
	factIDs := make([]string, 0)
	messageRefs := make([]string, 0)
	for _, raw := range row.EvidenceRefs {
		ref := strings.TrimSpace(stringValue(raw))
		switch {
		case strings.HasPrefix(ref, "message:"):
			messageRefs = append(messageRefs, ref)
		case strings.HasPrefix(ref, "fact:"):
			factIDs = append(factIDs, strings.TrimPrefix(ref, "fact:"))
		case strings.HasPrefix(ref, "sequence:"):
			sequence, err := strconv.Atoi(strings.TrimPrefix(ref, "sequence:"))
			if err != nil || sequence < 0 {
				continue
			}
			var id string
			err = tx.QueryRow(ctx, `SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence=$2`, command.OwnerFluctlightID, sequence).Scan(&id)
			if err == nil {
				factIDs = append(factIDs, id)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
	}
	if len(factIDs) > 0 {
		rows, err := tx.Query(ctx, `SELECT id FROM public.conversation_messages WHERE source_fact_id=ANY($1::text[])`, sortedUniqueStrings(factIDs))
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			messageRefs = append(messageRefs, "message:"+id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if len(messageRefs) == 0 {
		return nil
	}
	return invalidateConversationSummariesBySourceRefsTx(ctx, tx, command.OwnerFluctlightID, sortedUniqueStrings(messageRefs))
}
