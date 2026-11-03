# Eino payload modifier evidence

2026-10-14: Context7 library resolution selected /cloudwego/eino-ext (official, High). docs query confirmed WithRequestPayloadModifier transforms serialized JSON with context/messages. Source: https://github.com/cloudwego/eino-ext/blob/main/skills/eino-component/reference/model/openai.md . The returned snippets do not establish modifier composition.

Pinned source inspection: model/openai v0.1.13 option.go:59 delegates to acl/openai; acl/openai v0.1.17 option.go:73 assigns o.RequestPayloadModifier directly (replacement, not a chain); chat_model.go:723 invokes it with the current call context. Therefore observation is integrated in the existing normal/repair modifier, not appended as a second competing callback. HTTP/SSE tests must prove final policy and observation agree. No dependency upgrades.
