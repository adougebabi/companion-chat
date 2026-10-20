package core

// Directory metadata is deliberately distinct from full calling instructions.
// Named disclosure preserves Definition.Description and the complete schema.
// Do not shorten factual tool results or provider/native instructions here.
var capabilityDirectoryPurposes = map[string]string{
	"capability.catalog":                 "List available tools; request named tool details.",
	"capability.discover":                "Load selected tool schemas for the next request.",
	"capability.request":                 "Request a missing capability for Owner review.",
	"conversation.reply":                 "Send a private message and finish the turn.",
	"moment.publish":                     "Publish a Moment to the shared feed.",
	"media.image.generate":               "Generate an image from the requested subject.",
	"scene_event":                        "Change the character's own scene, activity or location.",
	"presence_event":                     "Record temporary interaction presence.",
	"affect_event":                       "Record a semantic affect event.",
	"memory_event":                       "Record or correct sourced lasting memories.",
	"active_memory_event":                "Record or close temporary facts.",
	"memory.recall":                      "Recall authorized memories and correction references.",
	"actor.inspect":                      "Read sourced Actor attributes and their history.",
	"actor.fact.record":                  "Record or correct sourced Actor attributes.",
	"relationship.lookup":                "Read a direct relationship with an authorized Actor.",
	"persona.detail":                     "Read current persona details, habits and appearance.",
	"persona.takeover":                   "Apply a declared temporary persona takeover.",
	"persona.switch":                     "Apply a declared persona profile switch.",
	"appearance.style":                   "Set or clear a temporary hairstyle.",
	"wardrobe.inspect":                   "Inspect inventory, outfits, wearing or obtainable items.",
	"wardrobe.wear":                      "Change clothing using available inventory items.",
	"wardrobe.borrow":                    "Record an authorized clothing loan or try-on.",
	"wardrobe.return":                    "Return borrowed items.",
	"wardrobe.outfit.save":               "Save an outfit from existing inventory items.",
	"item.use":                           "Start or stop using an owned ordinary object.",
	"habit.inspect":                      "Read the speaking profile's effective habits.",
	"habit.decide":                       "Change the speaking profile's ordinary habits.",
	"intention.inspect":                  "Read intentions and their status.",
	"intention.decide":                   "Create or change intentions and their status.",
	"schedule.inspect":                   "Read the accepted local-day schedule.",
	"schedule.edit":                      "Move, revise or cancel a schedule item.",
	"schedule.replan":                    "Replan current and future local-day intervals.",
	"intention.schedule":                 "Schedule a Goal action or request planning.",
	"life.activity.start":                "Start a timed virtual activity.",
	"life.activity.advance":              "Extend or resolve a started virtual activity.",
	"world.npc.encounter":                "Contact or pass on an actual NPC co-presence opportunity.",
	"goal.inspect":                       "Read Goals, results and evidence.",
	"goal.decide":                        "Issue Owner commands to create or change Goals.",
	"goal.evaluate":                      "Queue Goal evaluation for relevant new evidence.",
	"goal.review":                        "Queue a local-day Goal review.",
	"goal_planner.query":                 "Read authoritative planning context and history.",
	"goal_planner.commit":                "Commit Goal planning changes atomically.",
	"goal.evaluation.submit":             "Submit Goal judgments and review.",
	"goal.object.submit":                 "Submit Stage or Commitment judgments.",
	"goal.plan.submit":                   "Submit the current plan for an active Goal.",
	"visual_identity.initialize":         "Start the durable Visual Identity workflow.",
	"visual_identity.generate_candidate": "Generate a Visual Identity candidate.",
	"visual_identity.commit_review":      "Persist a real-image identity review.",
	"visual_identity.finalize":           "Finalize the accepted Visual Identity.",
}

func capabilityDirectoryPurpose(definition CapabilityDefinition) string {
	if purpose := capabilityDirectoryPurposes[definition.Name]; purpose != "" {
		return purpose
	}
	// Custom installations keep their actual purpose. Built-ins must have
	// deliberate short metadata (covered independently by the catalog test).
	return definition.Description
}
