package core

import (
	"strings"
	"testing"
	"time"
)

// A corrected Memory can refer to a historical message without changing that
// message. Old and rebuilt summaries must not regain its superseded conclusion.
func TestOwnerMemoryCorrectionFencesOldSummarySources(t *testing.T) {
	for _, test := range []struct {
		sourceRef    string
		confirm      bool
		legacy       bool
		legacyParent bool
	}{{"message:summary-message-1", false, false, false}, {"fact:summary-source-fact", false, false, false}, {"summary-source-fact", false, false, false}, {"sequence:1", false, false, false}, {"conversation_summary:summary-old", false, false, false}, {"memory:base", false, false, false}, {"conversation_summary:summary-old", true, false, false}, {"memory:base", true, false, false}, {"conversation_summary:summary-old", true, true, false}, {"memory:base", true, true, false}, {"memory:base", false, false, true}, {"conversation_summary:summary-old", false, true, false}} {
		name := test.sourceRef
		if test.legacyParent {
			name += "/legacy-parent"
		} else if test.legacy && !test.confirm {
			name += "/legacy-first"
		} else if test.legacy {
			name += "/legacy-confirm"
		} else if test.confirm {
			name += "/confirmed-first"
		}
		t.Run(name, func(t *testing.T) {
			sourceRef := test.sourceRef
			ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
			seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 4)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('summary-source-fact',$1,1,'conversation.turn','{"text":"原始交流"}','source-cause','source-correlation','source-key',now(),'processed')`, fid); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET source_fact_id='summary-source-fact' WHERE id='summary-message-1'`); err != nil {
				t.Fatal(err)
			}
			messages, err := readConversationSummaryMessages(ctx, repo.Pool(), conversation, 1, 4, 4)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose) VALUES('summary-fixture-provider','openai_compatible','http://fixture.invalid','fixture')`); err != nil {
				t.Fatal(err)
			}
			for _, item := range []struct {
				id, text string
				from, to int
			}{{"summary-old", "旧结论", 1, 2}, {"summary-unrelated", "另一段交流", 3, 4}} {
				window := messages[item.from-1 : item.to]
				_, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active',1,'summary-fixture-provider','fixture-model',$1,'fixture','fixture','fixture',$1,$1)`, item.id, fid, conversation, item.from, item.to, jsonBytes(conversationSummarySourceRefs(window)), conversationSummarySourceDigest(window), item.text)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_runtime_summaries(owner_fluctlight_id,conversation_id,revision,covered_through,summary,source_digest,status) VALUES($1,$2,1,2,'旧累计结论','old-digest','active')`, fid, conversation); err != nil {
				t.Fatal(err)
			}
			app := &App{DB: repo}
			command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{sourceRef}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-source-fact", SemanticReason: "源消息的原判断", IdempotencyKey: "summary-fence-create", Semantic: &MemorySemanticInput{Type: "semantic", Content: "旧结论", Confidence: 0.8, Importance: 0.8}}
			var baseMemoryID string
			switch sourceRef {
			case "conversation_summary:summary-old":
				var fingerprint string
				if err := repo.Pool().QueryRow(ctx, `SELECT public.conversation_summary_source_fingerprint('summary-old')`).Scan(&fingerprint); err != nil {
					t.Fatal(err)
				}
				command.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{sourceRef: {Kind: "conversation_summary", ID: "summary-old", Revision: 1, Fingerprint: fingerprint}}
			case "memory:base":
				base := command
				base.EvidenceRefs = []string{"message:summary-message-1"}
				base.IdempotencyKey = "summary-fence-base"
				base.Semantic = &MemorySemanticInput{Type: "semantic", Content: "同来源另一条事实", Confidence: 0.8, Importance: 0.8}
				if test.legacyParent {
					var fingerprint string
					if err := repo.Pool().QueryRow(ctx, `SELECT public.conversation_summary_source_fingerprint('summary-old')`).Scan(&fingerprint); err != nil {
						t.Fatal(err)
					}
					base.EvidenceRefs = []string{"conversation_summary:summary-old"}
					base.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{"conversation_summary:summary-old": {Kind: "conversation_summary", ID: "summary-old", Revision: 1, Fingerprint: fingerprint}}
				}
				base.RequestDigest = memoryCommandDigest(base)
				stored := applyMemoryLifecycleTestCommand(t, ctx, app, base)
				baseMemoryID = stored.MemoryID
				if test.legacyParent {
					row, err := app.readOwnedMemoryAuthorityRow(ctx, owner, stored.MemoryID)
					if err != nil {
						t.Fatal(err)
					}
					confirm := buildOwnerMemoryCommand(row, owner, MemoryConfirm, stored.Revision, []string{"upstream-owner-confirmation"}, nil, "确认上游", nil, nil)
					stored, err = app.applyOwnerMemoryCommand(ctx, confirm)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := repo.Pool().Exec(ctx, `UPDATE public.memory_source_links SET source_kind='legacy',source_id=NULL,source_revision=NULL,source_fingerprint=NULL,status='unknown' WHERE memory_id=$1 AND memory_revision=$2 AND source_ref='conversation_summary:summary-old'`, stored.MemoryID, stored.Revision); err != nil {
						t.Fatal(err)
					}
				}
				var digest string
				if err := repo.Pool().QueryRow(ctx, `SELECT request_digest FROM public.memories WHERE id=$1`, stored.MemoryID).Scan(&digest); err != nil {
					t.Fatal(err)
				}
				command.FrozenEvidenceSources = map[string]FrozenMemoryEvidenceSource{sourceRef: {Kind: "memory", ID: stored.MemoryID, Revision: stored.Revision, Fingerprint: digest}}
			}
			command.RequestDigest = memoryCommandDigest(command)
			created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
			query := ConversationSummaryQuery{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Limit: 4, MaxRunes: conversationSummaryDefaultBudget}
			before, err := app.retrieveRuntimeSummary(ctx, query, nil)
			if err != nil || len(before.Items) != 1 {
				t.Fatalf("baseline runtime summary unavailable: %#v %v", before, err)
			}
			var legacyDerivedID string
			var legacyDerivedRevision int
			zero := created.Revision
			if test.legacy && !test.confirm {
				derived := command
				derived.IdempotencyKey = "summary-legacy-derived"
				derived.Semantic = &MemorySemanticInput{Type: "semantic", Content: "旧摘要的派生结论", Confidence: 0.8, Importance: 0.8}
				derived.RequestDigest = memoryCommandDigest(derived)
				derivedResult := applyMemoryLifecycleTestCommand(t, ctx, app, derived)
				legacyDerivedID, legacyDerivedRevision = derivedResult.MemoryID, derivedResult.Revision
				// Reconstruct a migrated source with no earlier typed binding.
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.memory_source_links SET source_kind='legacy',source_id=NULL,source_revision=NULL,source_fingerprint=NULL,status='unknown' WHERE memory_id=$1 AND source_ref=$2`, created.MemoryID, sourceRef); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.memories SET provenance_status='legacy_unknown' WHERE id=$1`, created.MemoryID); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.memory_source_links SET source_kind='legacy',source_id=NULL,source_revision=NULL,source_fingerprint=NULL,status='unknown' WHERE memory_id=$1`, legacyDerivedID); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.memories SET provenance_status='legacy_unknown' WHERE id=$1`, legacyDerivedID); err != nil {
					t.Fatal(err)
				}
				if valid, err := memorySourcesStillValid(ctx, repo.Pool(), legacyDerivedID, legacyDerivedRevision, "legacy_unknown"); err != nil || !valid {
					t.Fatalf("uncorrected legacy summary was rejected: valid=%t err=%v", valid, err)
				}
			}
			if test.confirm {
				row, err := app.readOwnedMemoryAuthorityRow(ctx, owner, created.MemoryID)
				if err != nil {
					t.Fatal(err)
				}
				confirm := buildOwnerMemoryCommand(row, owner, MemoryConfirm, zero, []string{"owner-confirmation-before-correction"}, nil, "确认而非更正", nil, nil)
				confirmed, err := app.applyOwnerMemoryCommand(ctx, confirm)
				if err != nil {
					t.Fatal(err)
				}
				zero = confirmed.Revision
				var kind, status string
				if err := repo.Pool().QueryRow(ctx, `SELECT source_kind,status FROM public.memory_source_links WHERE memory_id=$1 AND memory_revision=$2 AND source_ref=$3`, created.MemoryID, zero, sourceRef).Scan(&kind, &status); err != nil || kind != strings.SplitN(sourceRef, ":", 2)[0] || status != "valid" {
					t.Fatalf("confirmation lost typed source: kind=%q status=%q err=%v", kind, status, err)
				}
				if test.legacy {
					// Reconstruct a pre-fix confirmation that lost the typed source;
					// the historical read fence must still consult older revisions.
					if _, err := repo.Pool().Exec(ctx, `UPDATE public.memory_source_links SET source_kind='legacy',source_id=NULL,source_revision=NULL,source_fingerprint=NULL,status='unknown' WHERE memory_id=$1 AND memory_revision=$2 AND source_ref=$3`, created.MemoryID, zero, sourceRef); err != nil {
						t.Fatal(err)
					}
				}
				before, err = app.retrieveRuntimeSummary(ctx, query, nil)
				if err != nil || len(before.Items) != 1 {
					t.Fatalf("confirmation alone hid the summary: %#v %v", before, err)
				}
			}
			if _, err := app.ReviseMemory(ctx, owner, created.MemoryID, "更正后的结论", &zero, []any{"owner-confirmation"}); err != nil {
				t.Fatal(err)
			}
			if legacyDerivedID != "" {
				if valid, err := memorySourcesStillValid(ctx, repo.Pool(), legacyDerivedID, legacyDerivedRevision, "legacy_unknown"); err != nil || valid {
					t.Fatalf("legacy derived memory bypassed correction: valid=%t err=%v", valid, err)
				}
				for _, scope := range []struct{ owner, conversation string }{{"other-owner", conversation}, {fid, "other-conversation"}} {
					if blocked, err := summaryContainsCorrectedMemorySource(ctx, repo.Pool(), scope.owner, scope.conversation, 1, 2); err != nil || blocked {
						t.Fatalf("legacy correction escaped scope: blocked=%t err=%v", blocked, err)
					}
				}
			}
			runtime, err := app.retrieveRuntimeSummary(ctx, query, nil)
			if err != nil || len(runtime.Items) != 0 || len(runtime.Trace.Selections) != 1 || runtime.Trace.Selections[0].Reason != "source_memory_corrected" {
				t.Fatalf("corrected source remained in runtime summary: %#v %v", runtime, err)
			}
			writerSummaries, err := app.replyRuntimeSummaries(ctx, ContextProjection{OwnerActorID: owner, FluctlightID: fid, ConversationID: conversation})
			if err != nil || len(writerSummaries) != 0 {
				t.Fatalf("Writer still received corrected-source summary: %#v %v", writerSummaries, err)
			}
			episodes, err := app.retrieveConversationSummaries(ctx, query)
			if err != nil || len(episodes.Items) != 1 || stringValue(episodes.Items[0]["summary"]) != "另一段交流" {
				t.Fatalf("corrected episode was exposed or unrelated episode lost: %#v %v", episodes, err)
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_runtime_summaries SET status='active',revision=revision+1,summary='后台重复的旧结论' WHERE owner_fluctlight_id=$1 AND conversation_id=$2`, fid, conversation); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := app.retrieveRuntimeSummary(ctx, query, nil)
			if err != nil || len(rebuilt.Items) != 0 {
				t.Fatalf("same-source rebuild resurrected old summary: %#v %v", rebuilt, err)
			}
			one := zero + 1
			if _, err := app.ReviseMemory(ctx, owner, created.MemoryID, "旧结论", &one, []any{"owner-confirmation-again"}); err != nil {
				t.Fatal(err)
			}
			afterABA, err := app.retrieveRuntimeSummary(ctx, query, nil)
			if err != nil || len(afterABA.Items) != 0 {
				t.Fatalf("changing the text back resurrected an unfenced summary: %#v %v", afterABA, err)
			}
			var rawText string
			if err := repo.Pool().QueryRow(ctx, `SELECT text FROM public.conversation_messages WHERE id='summary-message-1'`).Scan(&rawText); err != nil || rawText != "raw-summary-message-1" {
				t.Fatalf("raw history changed: %q %v", rawText, err)
			}
			if baseMemoryID != "" {
				var content, status string
				if err := repo.Pool().QueryRow(ctx, `SELECT content,status FROM public.memories WHERE id=$1`, baseMemoryID).Scan(&content, &status); err != nil || content != "同来源另一条事实" || status != "active" {
					t.Fatalf("independent source Memory changed: content=%q status=%q err=%v", content, status, err)
				}
			}
		})
	}
}

func TestMemoryConfirmationDoesNotReviveChangedInheritedSource(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
	app := &App{DB: repo}
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"message:summary-message-1"}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-message-1", SemanticReason: "原始来源", IdempotencyKey: "confirm-stale-source-create", Semantic: &MemorySemanticInput{Type: "semantic", Content: "真实待确认记忆", Confidence: 0.8, Importance: 0.8}}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='来源已改变' WHERE id='summary-message-1'`); err != nil {
		t.Fatal(err)
	}
	row, err := app.readOwnedMemoryAuthorityRow(ctx, owner, created.MemoryID)
	if err != nil {
		t.Fatal(err)
	}
	confirm := buildOwnerMemoryCommand(row, owner, MemoryConfirm, created.Revision, []string{"owner-confirmation"}, nil, "新的确认", nil, nil)
	confirmed, err := app.applyOwnerMemoryCommand(ctx, confirm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='raw-summary-message-1' WHERE id='summary-message-1'`); err != nil {
		t.Fatal(err)
	}
	var status, provenance string
	var fingerprintMatches bool
	if err := repo.Pool().QueryRow(ctx, `SELECT l.status,m.provenance_status,public.memory_source_is_live(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint) FROM public.memory_source_links l JOIN public.memories m ON m.id=l.memory_id WHERE l.memory_id=$1 AND l.memory_revision=$2 AND l.source_ref='message:summary-message-1'`, created.MemoryID, confirmed.Revision).Scan(&status, &provenance, &fingerprintMatches); err != nil {
		t.Fatal(err)
	}
	if status != "invalid" || provenance != "partial" || !fingerprintMatches {
		t.Fatalf("restored bytes revived stale evidence: status=%q provenance=%q fingerprintMatches=%t", status, provenance, fingerprintMatches)
	}
}

func TestSummaryCorrectionFenceKeepsOwnerConversationAndWindowScope(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 4)
	seedTurnConversation(t, ctx, repo, "other-owner", "other-fluctlight", "other-conversation")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('other-message-1','other-conversation',1,'other-owner','user','其他会话','[]','other-key-1')`); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"message:summary-message-1"}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-message-1", SemanticReason: "本会话来源", IdempotencyKey: "scope-fence-create", Semantic: &MemorySemanticInput{Type: "semantic", Content: "本会话原判断", Confidence: 0.8, Importance: 0.8}}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	revision := created.Revision
	if _, err := app.ReviseMemory(ctx, owner, created.MemoryID, "本会话更正", &revision, []any{"owner-confirmation"}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		owner, conversation string
		from, to            int
		want                bool
	}{{fid, conversation, 1, 2, true}, {fid, conversation, 3, 4, false}, {fid, "other-conversation", 1, 2, false}, {"other-fluctlight", conversation, 1, 2, false}, {"other-fluctlight", "other-conversation", 1, 2, false}} {
		got, err := summaryContainsCorrectedMemorySource(ctx, repo.Pool(), test.owner, test.conversation, test.from, test.to)
		if err != nil || got != test.want {
			t.Fatalf("scope owner=%q conversation=%q window=%d..%d: got=%t want=%t err=%v", test.owner, test.conversation, test.from, test.to, got, test.want, err)
		}
	}
}
