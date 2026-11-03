# Broad isolated DB regression failures

This run had live Provider disabled. Do not weaken product assertions. Some fixtures encode retired loops; migrate those to native Tool/final response contracts. Full source log is runs/integration-go-db-01/output.log.

TestAuthorizedPersistentSwitchLoadsTheNewPersonaNextTurn
phase11_verification_test.go:52: adk_final_output_invalid: field "personality_decision": required field "confidence" is missing

TestConversationRejectsLifeContextChangeBetweenDecisionAndSettlement
life_context_test.go:392: stale conversation err=provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations

TestConversationTurnWithNativeImageAndReplyCommitsBothOutputs
conversation_mixed_media_reply_test.go:44: mixed turn failed: provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations

TestDeclaredRulesReachTheProductionTurn
turn_takeover_chain_test.go:283: adk_final_output_invalid: additional property "response_mode" is not allowed

TestDirectConversationMessageIsDurableDuringCognitionAndNoReplyCannotComplete
conversation_delivery_regression_test.go:122: no-reply cognition was silently completed: result=core.TurnResult{UserMessage:map[string]interface {}(nil), Assistant:map[string]interface {}(nil), MediaIntentID:"", TurnID:"", CorrelationID:""} err=adk_final_output_invalid: additional property "tool_calls" is not allowed assistant=0 inbox=failed completed_frozen=0

TestDirectConversationReplyCanonicalToolCallWithEmptyStructuredFieldsCommitsMessage
conversation_delivery_regression_test.go:362: canonical conversation.reply turn failed: provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations

TestDirectConversationReplyToolWithoutAppraisalCommitsBothMessages
conversation_delivery_regression_test.go:304: reply-tool turn err=provider request failed: adk_run: [NodeRunError] run node[ChatModel] post processor fail: concat stream reader fail: stream reader is empty, concat fail

TestDirectConversationToolCallsExecuteIndependentlyWithoutVisibleReply
conversation_delivery_regression_test.go:321: tool-only direct turn failed: provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations

TestEinoADKStructuredContentToolCallsAreNotExecuted
conversation_mixed_media_reply_test.go:67: structured pseudo-tool turn did not fail clearly: result=core.TurnResult{UserMessage:map[string]interface {}(nil), Assistant:map[string]interface {}(nil), MediaIntentID:"", TurnID:"", CorrelationID:""} err=adk_final_output_invalid: additional property "response_mode" is not allowed

TestEnqueueTurnFactClaimedReplaysCommittedAssistant
cognition_claim_test.go:102: ERROR: relation "public.cognition_inbox" does not exist (SQLSTATE 42P01)

TestEnsureWakeUpIntentsRepairsExistingLiveFluctlight
wakeup_intents_test.go:406: ERROR: relation "public.fluctlights" does not exist (SQLSTATE 42P01)

TestIndependentToolE2EPresenceEvent


TestIndependentToolE2EPresenceEvent/business_failure
independent_tools_e2e_test.go:521: Tool failure code="capability_prepare_failed" receipt=core.ToolExecutionReceipt{OperationID:"presence-expired-presence_1790064129123713000", NativeToolCallID:"", ExecutionCallID:"direct_call_09e99a54b3cd2d980d21958ad47694c0", Result:capability.CapabilityResult{CallID:"direct_call_09e99a54b3cd2d980d21958ad47694c0", CapabilityName:"presence_event", Status:"failed", Output:map[string]interface {}{"detail":"capability execution failed: presence_expiration_invalid: invalid capability arguments"}, ErrorCode:"presence_expiration_invalid", Retryable:false, ProviderRequestID:"", CorrelationID:"capability:direct_call_09e99a54b3cd2d980d21958ad47694c0", Duration:0, RequiredContext:[]capability.ContextSlot(nil)}, Replayed:false} err=capability execution failed: presence_expiration_invalid: invalid capability arguments

TestIndependentToolE2ESceneEvent


TestIndependentToolE2ESceneEvent/business_failure
independent_tools_e2e_test.go:521: Tool failure code="capability_prepare_failed" receipt=core.ToolExecutionReceipt{OperationID:"scene-start-while-active-scene_1790064128304356000", NativeToolCallID:"", ExecutionCallID:"direct_call_8d690e53b20e465a1474a0e7aade6b38", Result:capability.CapabilityResult{CallID:"direct_call_8d690e53b20e465a1474a0e7aade6b38", CapabilityName:"scene_event", Status:"failed", Output:map[string]interface {}{"detail":"capability execution failed: scene_start_requires_no_active_event: resource revision conflict"}, ErrorCode:"scene_start_requires_no_active_event", Retryable:false, ProviderRequestID:"", CorrelationID:"capability:direct_call_8d690e53b20e465a1474a0e7aade6b38", Duration:0, RequiredContext:[]capability.ContextSlot(nil)}, Replayed:false} err=capability execution failed: scene_start_requires_no_active_event: resource revision conflict

TestIndependentToolE2EScheduleReplan
independent_tools_e2e_test.go:168: FLUCTLIGHT_LIVE_PROVIDER_TEST=1 is required for the real schedule.replan Tool E2E

TestInvalidCandidateFailsBeforeTheJudge
turn_takeover_chain_test.go:542: no rows in result set

TestLogicalInvocationsVsPhysicalAttempts


TestLogicalInvocationsVsPhysicalAttempts/a_deterministic_turn_spends_one_attempt_per_logical_stage
turn_chain_budget_test.go:168: adk_final_output_invalid: additional property "response_mode" is not allowed

TestLogicalInvocationsVsPhysicalAttempts/a_schema-invalid_judge_reply_is_coerced,_not_reported_as_invalid_output
turn_chain_budget_test.go:318: adk_final_output_invalid: additional property "response_mode" is not allowed

TestLogicalInvocationsVsPhysicalAttempts/a_timeout_judge_keeps_the_candidate_at_bounded_cost
turn_chain_budget_test.go:222: adk_final_output_invalid: additional property "response_mode" is not allowed

TestLogicalInvocationsVsPhysicalAttempts/a_unavailable_judge_keeps_the_candidate_at_bounded_cost
turn_chain_budget_test.go:222: adk_final_output_invalid: additional property "response_mode" is not allowed

TestLogicalInvocationsVsPhysicalAttempts/an_over-budget_judge_packet_is_refused_before_the_network
turn_chain_budget_test.go:276: adk_final_output_invalid: additional property "tool_calls" is not allowed

TestMainGenerationBudgetNeverExceedsTwo


TestMainGenerationBudgetNeverExceedsTwo/approved_Judge_replaces_the_candidate
turn_takeover_chain_test.go:688: adk_final_output_invalid: additional property "response_mode" is not allowed

TestMainGenerationBudgetNeverExceedsTwo/declined_Judge_keeps_the_candidate
turn_takeover_chain_test.go:688: adk_final_output_invalid: additional property "response_mode" is not allowed

TestMainGenerationBudgetNeverExceedsTwo/no_takeover_rule_pays_for_nothing_extra
turn_takeover_chain_test.go:688: adk_final_output_invalid: additional property "response_mode" is not allowed

TestMixedQueryActionBatchRemainsOneAgentRun
turn_takeover_chain_test.go:618: provider_cognitive_state_transition_forbidden

TestMultiProfileScopeMatrixReadFollowsTheSpeaker
takeover_scope_matrix_test.go:111: adk_final_output_invalid: additional property "response_mode" is not allowed

TestNativeCognitionCycleGuardBoundsCapabilityProducedLifeFacts
project_health_transaction_integration_test.go:1122: resource not found

TestNativeCognitionRequiredFailureReplaysFrozenDecisionWithoutSecondProviderCall
project_health_transaction_integration_test.go:694: first attempt calls provider=1 prepare=0 tx=0

TestNextTurnAfterTakeoverRestoresThePersistentPerspective
takeover_scope_matrix_test.go:267: adk_final_output_invalid: additional property "response_mode" is not allowed

TestOverlayFailureKeepsAByExplicitContractInProductionTurn
turn_takeover_f05_test.go:256: adk_final_output_invalid: additional property "response_mode" is not allowed

TestPersistentSwitchAssessmentRunsAfterTheCandidateKeepsTheTurn
persistent_switch_assessment_test.go:31: adk_final_output_invalid: additional property "response_mode" is not allowed

TestPostgresInitializationSourceMigrationFromPreviousHead
runner_test.go:123: initialization source migration head="0034_tool_execution_source" sources=0xf02c0286450 links=0xf02c0286460

TestPostgresModelRunLateTerminalCallbackIsAnIdempotentNoop
diagnostics_test.go:250: ERROR: relation "public.diagnostic_model_runs" does not exist (SQLSTATE 42P01)

TestPostgresProviderTimeoutPersistsOneTypedTerminalState
diagnostics_test.go:299: ERROR: relation "public.diagnostic_model_runs" does not exist (SQLSTATE 42P01)

TestPostgresWakeUpClockRecoversLostRedisAndDeduplicatesRelease
wakeup_intents_test.go:469: ERROR: relation "public.fluctlights" does not exist (SQLSTATE 42P01)

TestProcessReflectionAppliesAllMemoryCandidateV2OperationsAtomically
memory_lifecycle_test.go:499: all-operation Reflection=map[string]interface {}(nil) err=adk_final_output_invalid: field "memory_candidates": item 0: required field "confidence" is missing

TestProcessReflectionV2AdvancesGoalOnlyFromBoundCompletedOutcome
reflection_goal_progress_v2_test.go:82: Provider request omitted Goal/Outcome refs: {"enable_thinking":true,"max_completion_tokens":4096,"messages":[{"role":"system","content":"# 运行协议\n\n1. 语言：自然语言用中文，协议/字面量保持原文。\n2. 约束优先级：core_persona（硬约束）\u003e developing_self（带证据线索）\u003e current_state（当前事实）。\n3. 上下文绑定：决策与工具参数必须锚定 context 起始快照以及之后真实 Tool 已提交的结果（scene, activity, location, mood, appearance）；后续查询和已提交结果代表更新后的事实。除 actor_user 明确要求外，禁止擅自变更场景；actor_user 显式变更时标明 context_override.explicit=true。\n4. Actor 语义：Human 与 Fluctlight 都是 Actor；消息发送者以 Actor 与关系上下文为准，不要把 transport role=user 当作 actor_user 身份。\n5. 认知与生成准则：\n   - 认知字段仅写简短摘要，禁止输出推理长文。\n   - cl
reflection_goal_progress_v2_test.go:97: Reflection result=map[string]interface {}(nil) err=provider request failed: adk_run: [NodeRunError] context deadline exceeded

TestRecentExactAssistantMessageTxSuppressesOnlyRecentExactText
proactive_delivery_test.go:37: ERROR: relation "public.conversation_heads" does not exist (SQLSTATE 42P01)

TestRecoveryDoesNotReplayCompletedStages


TestRecoveryDoesNotReplayCompletedStages/a_persisted_judge_kept_a_promotes_the_candidate_without_a_new_verdict
turn_chain_budget_test.go:397: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryDoesNotReplayCompletedStages/a_persisted_takeover_verdict_finishes_the_second_generation_only
turn_chain_budget_test.go:425: the first attempt must consult the Judge once, got 0

TestRecoveryDoesNotReplayCompletedStages/a_turn_admitted_into_the_execution_window_is_resumed_idempotently
turn_chain_budget_test.go:386: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryDoesNotReplayCompletedStages/an_already_decided_winner_is_executed,_never_re-arbitrated
turn_chain_budget_test.go:364: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryDoesNotReplayCompletedStages/an_already_generated_takeover_reply_is_promoted,_never_regenerated
turn_chain_budget_test.go:375: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryDoesNotReplayCompletedStages/an_unarbitrated_candidate_arbitrates_exactly_once
turn_chain_budget_test.go:438: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryFromAFrozenReArbitratesExactlyOnce
turn_takeover_recovery_test.go:342: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryFromArbitrationKeptAPromotesTheCandidate
turn_takeover_recovery_test.go:225: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryFromBFrozenExecutesTheTakeoverReply
turn_takeover_recovery_test.go:204: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryFromExecutingResumesWithoutANewVerdict
turn_takeover_recovery_test.go:312: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryFromPendingTakeoverContinuesWithoutReJudging
turn_takeover_recovery_test.go:271: the first attempt must consult the Judge once, got 0

TestRecoveryFromPendingTakeoverRebuildsRuleFromFrozenRecord
turn_takeover_f04_test.go:96: no rows in result set

TestRecoveryFromPendingTakeoverResumesFromFrozenRuleNotCurrentRules
turn_takeover_f04_test.go:38: no rows in result set

TestRecoveryFromWinnerReadyExecutesTheDecidedWinner
turn_takeover_recovery_test.go:179: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRecoveryWithExistingAssistantIsAReplay
turn_takeover_recovery_test.go:376: adk_final_output_invalid: additional property "tool_calls" is not allowed

TestRedisConsumerPublishesAndDeduplicates
redis_pipeline_test.go:55: ERROR: relation "public.platform_outbox_events" does not exist (SQLSTATE 42P01)

TestRedisConsumerReclaimsPendingDelivery
redis_pipeline_test.go:176: ERROR: relation "public.platform_consumer_failures" does not exist (SQLSTATE 42P01)

TestRedisConsumerRecordsPoisonEvent
redis_pipeline_test.go:131: ERROR: relation "public.platform_consumer_failures" does not exist (SQLSTATE 42P01)

TestRedisConsumerSequenceGapReleasesDatabaseConnection
redis_pipeline_test.go:218: ERROR: relation "public.platform_consumer_heads" does not exist (SQLSTATE 42P01)

TestReflectionMemoryApplyRollsBackWithProposalAndWatermark
memory_lifecycle_test.go:585: malformed Reflection err=adk_final_output_invalid: field "memory_candidates": item 0: required field "operation" is missing

TestRejectedCandidateLeavesNoFactTrail
turn_takeover_recovery_test.go:477: adk_final_output_invalid: additional property "loss" is not allowed

TestRelationshipInteractionSettlementFollowsTheReplyOwner


TestRelationshipInteractionSettlementFollowsTheReplyOwner/judge_approved_B
takeover_scope_matrix_test.go:213: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRelationshipInteractionSettlementFollowsTheReplyOwner/judge_kept_A
takeover_scope_matrix_test.go:213: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRelationshipInteractionSettlementFollowsTheReplyOwner/shared_fallback_for_A
takeover_scope_matrix_test.go:213: adk_final_output_invalid: additional property "response_mode" is not allowed

TestRelationshipInteractionSettlementFollowsTheReplyOwner/shared_fallback_for_B
takeover_scope_matrix_test.go:213: adk_final_output_invalid: additional property "response_mode" is not allowed

TestReleaseCognitionClaimRequeuesOwnedClaim
cognition_claim_test.go:45: ERROR: relation "public.cognition_inbox" does not exist (SQLSTATE 42P01)

TestRootVisibleTextWithoutReplyInvocationIsAccepted
turn_takeover_f01_test.go:36: a root-visible-text candidate without a reply invocation must be accepted: adk_final_output_invalid: additional property "response_mode" is not allowed

TestSameObservationDifferentSceneChangesDecisionPlannerOutcomeAndNextProjection
life_context_e2e_test.go:27: provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations

TestStaleSnapshotNeverOverwritesNewerState
turn_takeover_recovery_test.go:404: adk_final_output_invalid: additional property "response_mode" is not allowed

TestStreamTurnCanonicalConversationReplyWithEmptyStructuredFieldsEmitsAssistantFrame
conversation_delivery_regression_test.go:410: canonical conversation.reply stream frames = []map[string]interface {}{map[string]interface {}{"payload":map[string]interface {}{"correlation_id":"turn:canonical-stream-turn-1", "message":map[string]interface {}{"attachment_refs":[]interface {}{}, "author_actor_id":"canonical-stream-owner", "conversation_id":"canonical-stream-conversation", "created_at":"2026-09-22T08:01:51.923863Z", "id":"message_11a083ea5079bd90ccebebda26ea8cfb", "kind":"user", "sequence":1, "text":"你忙完了吗？"}}, "sequence":0, "turn_id":"canonical-stream-turn-1", "type":"action_result"}, map[string]interface {}{"payload":map[string]interface {}{"code":"conversation_settlement_failed", "status":"failed"}, "sequence":1, "turn_id":"canonical-stream-turn-1", "type":"error"}}

TestStreamTurnWithNativeImageAndReplyEmitsAssistantFrame
conversation_mixed_media_reply_test.go:106: stream frames = []map[string]interface {}{map[string]interface {}{"payload":map[string]interface {}{"correlation_id":"turn:mixed-turn-stream-1", "message":map[string]interface {}{"attachment_refs":[]interface {}{}, "author_actor_id":"mixed-owner-stream", "conversation_id":"mixed-conversation-stream", "created_at":"2026-09-22T08:01:56.554387Z", "id":"message_7df56eda66539b2de91029aa69332e78", "kind":"user", "sequence":1, "text":"给我看看"}}, "sequence":0, "turn_id":"mixed-turn-stream-1", "type":"action_result"}, map[string]interface {}{"payload":map[string]interface {}{"code":"conversation_settlement_failed", "status":"failed"}, "sequence":1, "turn_id":"mixed-turn-stream-1", "type":"error"}}

TestTakeoverJudgeApprovesExecutesOnlyTheTakeoverReply
turn_takeover_chain_test.go:392: adk_final_output_invalid: field "personality_decision": required field "confidence" is missing

TestTakeoverJudgeDeclinesExecutesTheOriginalCandidate
turn_takeover_chain_test.go:337: adk_final_output_invalid: additional property "response_mode" is not allowed

TestTakeoverReplyIsGeneratedInTheReplyOwnerScope
turn_takeover_chain_test.go:469: adk_final_output_invalid: additional property "response_mode" is not allowed

TestTurnPathCostMeasurement
turn_path_cost_report_test.go:106: adk_final_output_invalid: additional property "response_mode" is not allowed

TestTurnStageBudgetNeverExceedsTwoMainGenerations


TestTurnStageBudgetNeverExceedsTwoMainGenerations/a_declined_judge_is_metered_outside_the_main_budget
turn_chain_budget_test.go:103: adk_final_output_invalid: additional property "response_mode" is not allowed

TestTurnStageBudgetNeverExceedsTwoMainGenerations/an_approved_judge_adds_exactly_one_second_generation
turn_chain_budget_test.go:103: adk_final_output_invalid: additional property "response_mode" is not allowed

TestTurnStageBudgetNeverExceedsTwoMainGenerations/no_rule_pays_for_one_generation_and_no_judge
turn_chain_budget_test.go:103: adk_final_output_invalid: additional property "response_mode" is not allowed

TestUnauthorizedTakeoverCandidateBFailsControlled
turn_takeover_f02_test.go:250: the takeover generation must run once before B is rejected, got 0

TestWakeUpAndDailyReviewPrepareAgainstModelVisibleLifeThenFailStale


TestWakeUpAndDailyReviewPrepareAgainstModelVisibleLifeThenFailStale/daily_review
life_context_surface_test.go:251: daily_review stale run err=provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations

TestWakeUpAndDailyReviewPrepareAgainstModelVisibleLifeThenFailStale/wake_up
life_context_surface_test.go:251: wake_up stale run err=provider request failed: adk_run: [NodeRunError] run node[ChatModel] pre processor fail: exceeds max iterations
