"""Pure compatibility of already-effective NATS 2.12.12 subject languages.

No authentication, signature, scope/template expansion, wildcard SUB command
acceptance or PUB/ACK delivery is attested. Caller supplies processed effective
permissions. Queue/dynamic-response and unsupported grammar fail closed.
"""
from collections import deque
import hashlib
import json

class PermissionError(ValueError):
    pass

MAX_STATES=8192
MAX_STATE_ITEMS=65536
MAX_WORK=1000000
MAX_NODES=65536
MAX_BYTES=1048576


def _fail():
    raise PermissionError('actor_permissions_unsupported_or_missing')


class _Budget:
    def __init__(self):
        self.work = self.states = self.items = self.nodes = self.bytes = 0

    def spend(self, amount):
        self.work += amount
        if self.work > MAX_WORK: _fail()

    def retain(self, state):
        self.states += 1; self.items += 1 + len(state[1]) + len(state[2])
        if self.states > MAX_STATES or self.items > MAX_STATE_ITEMS: _fail()


def _patterns(value, limit, budget, nullable=False):
    if value is None and nullable: return None
    if type(value) is not list or len(value) > limit: _fail()
    result = []
    for subject in value:
        if not isinstance(subject, str) or not subject: _fail()
        length = len(subject.encode('utf-8'))
        budget.bytes += length
        if length > 4096 or budget.bytes > MAX_BYTES: _fail()
        if any(char.isspace() or ord(char) < 32 or ord(char) == 127 for char in subject): _fail()
        tokens = tuple(subject.split('.'))
        for index, token in enumerate(tokens):
            if (not token or '*' in token and token != '*' or '>' in token and
                    (token != '>' or index != len(tokens)-1)): _fail()
        budget.nodes += len(tokens) + 1
        if budget.nodes > MAX_NODES: _fail()
        result.append(tokens)
    return tuple(dict.fromkeys(result))


def _next(pattern, position, symbol):
    if position == len(pattern):
        return position if pattern[-1] == '>' else None
    token = pattern[position]
    return position + 1 if token in ('*', '>') or token == symbol else None


class _Union:
    def __init__(self, patterns):
        self.patterns = patterns
        self.initial = frozenset((index, 0) for index in range(len(patterns)))

    def advance(self, state, symbol, budget):
        budget.spend(len(state))
        result = set()
        for index, position in state:
            following = _next(self.patterns[index], position, symbol)
            if following is not None: result.add((index, following))
        return frozenset(result)

    def accepts(self, state, budget):
        budget.spend(len(state))
        return any(position == len(self.patterns[index]) for index, position in state)

    def literals(self, state, budget):
        budget.spend(len(state))
        return {self.patterns[index][position] for index, position in state
            if position < len(self.patterns[index]) and self.patterns[index][position] not in ('*', '>')}


def _compatible(required, allow, deny, budget):
    # Exact finite quotient of the infinite token alphabet: every currently
    # relevant literal has its own class, and None denotes all other literals.
    # Product exploration finds any required word denied or absent from UNION
    # allow. Tail '>' consumes one token before an accepting self-loop.
    initial = (0, allow.initial, deny.initial)
    pending = deque([initial]); seen = {initial}; budget.retain(initial)
    while pending:
        position, allowed, denied = pending.popleft(); budget.spend(1)
        if position == len(required):
            if deny.accepts(denied, budget) or not allow.accepts(allowed, budget): _fail()
            if required[-1] != '>': continue
        token = required[position] if position < len(required) else '>'
        symbols = {token} if token not in ('*', '>') else allow.literals(allowed, budget) | deny.literals(denied, budget) | {None}
        for symbol in symbols:
            budget.spend(1)
            following = (_next(required, position, symbol),
                allow.advance(allowed, symbol, budget), deny.advance(denied, symbol, budget))
            if following not in seen:
                budget.retain(following); seen.add(following); pending.append(following)


def _canonical(value):
    return json.dumps(value,sort_keys=True,separators=(',', ':'),ensure_ascii=False,allow_nan=False).encode('utf-8')

def evaluate(effective, required):
    """Require language inclusion in allow union with no deny intersection.

    Source: client.go setPermissions/pubAllowedFullCheck in pinned v2.12.12.
    PUB nil allow is unrestricted; PUB [] denies all. SUB nil AND [] are
    unrestricted (the server installs an allow sublist only when len>0).
    Original nil/empty values remain distinct in input hashes.
    """
    try:
        if (type(effective) is not dict or set(effective) != {'pub','sub','response'}
                or effective['response'] is not None or type(required) is not dict
                or set(required) != {'pub','sub'}): _fail()
        budget = _Budget(); parsed = {}
        for role in ('pub', 'sub'):
            supplied = effective[role]
            if type(supplied) is not dict or set(supplied) != {'allow','deny'}: _fail()
            allow = _patterns(supplied['allow'],4096,budget,True)
            deny = _patterns(supplied['deny'],4096,budget,True)
            if sum(len(supplied[key] or []) for key in ('allow','deny')) > 4096: _fail()
            need = _patterns(required[role],256,budget)
            unrestricted = allow is None or role == 'sub' and not allow
            parsed[role] = (need, _Union((('>',),) if unrestricted else allow), _Union(deny or ()))
        encoded = _canonical(effective); requested = _canonical(required)
        if len(encoded) + len(requested) > MAX_BYTES: _fail()
        for need, allow, deny in parsed.values():
            for pattern in need: _compatible(pattern,allow,deny,budget)
        return {'schema':'voice-nats-actor-permissions-v1',
            'effective_sha256':hashlib.sha256(encoded).hexdigest(),
            'required_sha256':hashlib.sha256(requested).hexdigest(),
            'effective_permission_language_compatible':True}
    except Exception:
        raise PermissionError('actor_permissions_unsupported_or_missing') from None
