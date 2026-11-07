"""Constrain nested Docker network modes to the private NATS fixture names."""

def apply(wrapper):
    old = '''    --network)
      [ "$n" -ge 2 ] && [ "$1" = none ] || { echo 'unsupported network option' >&2; exit 2; }
      shift; n=$((n-2)); continue ;;
    --net|--net=*|--network=*|--network-*|--pull|--pull=*)'''
    new = '''    --network)
      [ "$n" -ge 2 ] || { echo 'missing network' >&2; exit 2; }
      net=$1
      case "$net" in
        none|container:voice-nats-proof-*|container:voice-nats-canonical-*) ;;
        voice-nats-proof-*|voice-nats-canonical-*) suffix=${net##*-}; case "$suffix" in ''|*[!0-9]*) echo 'invalid private network' >&2; exit 2;; esac ;;
        *) echo 'unsupported network option' >&2; exit 2 ;;
      esac
      [ "$network_seen" = 0 ] || { echo 'duplicate network option' >&2; exit 2; }
      network_seen=1; set -- "$@" "$arg" "$net"; shift; n=$((n-2)); continue ;;
    --net|--net=*|--network=*|--pull|--pull=*)'''
    if wrapper.count(old) != 1: raise ValueError('network wrapper shape changed')
    wrapper = wrapper.replace('python_image=0', 'python_image=0\nnetwork_seen=0', 1).replace(old,new,1)
    old_tail = 'set -- run --pull=never --network none "$@"'
    new_tail = '[ "$network_seen" = 1 ] || { if [ "$python_image" = 1 ]; then set -- --network none "$@"; else echo "missing private network" >&2; exit 2; fi; }; set -- run --pull=never "$@"'
    if wrapper.count(old_tail) != 1: raise ValueError('network wrapper tail changed')
    return wrapper.replace(old_tail,new_tail,1)
