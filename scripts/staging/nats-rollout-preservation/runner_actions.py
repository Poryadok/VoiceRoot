"""Fixed high-level bridge actions exposed by the nonroot service."""
from __future__ import annotations

from bridge_state import BridgeState
from runner_rpc import RpcError


SUPPORTED_READ_ACTIONS = frozenset(('status',))
LEGACY_MUTATING_ACTIONS = frozenset((
    'prepare', 'prepare-repair', 'prepare-rollback', 'resume-cold-backup',
    'resume-cipher-upload', 'authorize-preserved-upload', 'prepare-expired-native',
    'prepare-expired-finish', 'authorize-expired-native', 'authorize-expired-finish',
    'finish-expired-native', 'authorize', 'finish', 'accept-provisional',
))


class Actions:
    """The runner owns this dispatch table; request data never selects code."""
    def __init__(self, state: BridgeState):
        self.state = state

    def execute(self, request: dict) -> dict:
        action = request.get('action')
        if action == 'status':
            return self.state.status(request['operation'])
        if action in LEGACY_MUTATING_ACTIONS:
            raise RpcError('runner_action_not_migrated')
        raise RpcError('runner_action_invalid')
