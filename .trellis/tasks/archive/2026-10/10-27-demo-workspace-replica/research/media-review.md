# Media independent review and resolution

- Visual contracts checked against visual_brief.go, visual_generation_queue.go, owner_visual_review.go and visual_identity_tools.go. Actual generate may return pending; supersedeSessionId does not prove the old session ended. Four fields and human review scores preserve their actual contracts.
- Fixed: old session/attempt review command becomes unreachable once the current attempt is decided. Persist original UI context and expose recovery independently of canReview. Accepted/regenerate remount regressions and browser original review replay pass.
- Fixed: revoked grants need original revoke command/outcome recovery entries on revoked rows. Actual status transition and regression verified.
- Fixed: display-avatar source_identity_changed must fall back to the name, not load current bytes by stored asset ID. Explicit new selections pin byte identity and only preview that chosen version. Regression verified.
- Main verification: global/character scoping, metadata dual CAS, unknown/readback recovery, source-version adoption, actual generation completion, independent avatar, reference confirmation, deletion acceptance/failure retry/unknown refresh, native returnTo and four-tab five-state browser evidence.
- Original upload controller retained. UI fixture evidence does not certify real storage, Core/DB, background workers or model execution.
