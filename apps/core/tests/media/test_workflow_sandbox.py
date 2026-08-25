import asyncio

from fluctlight_core.autonomy.workflows import AutonomyActionWorkflow
from fluctlight_core.media.workflows import MediaGenerationWorkflow
from temporalio.worker.workflow_sandbox import SandboxedWorkflowRunner
from temporalio.workflow import _Definition


def test_activity_dependencies_do_not_break_temporal_workflow_sandbox() -> None:
    async def prepare() -> None:
        runner = SandboxedWorkflowRunner()
        runner.prepare_workflow(_Definition.from_class(AutonomyActionWorkflow))
        runner.prepare_workflow(_Definition.from_class(MediaGenerationWorkflow))

    asyncio.run(prepare())
