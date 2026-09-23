#compdef idea
# zsh completion for idea. Install with:
#   idea completion zsh > "${fpath[1]}/_idea"
# or add `source <(idea completion zsh)` to ~/.zshrc after compinit.

_idea_lines() {
    reply=(${(f)"$(idea complete "$@" 2>/dev/null)"})
}

_idea_ids() {
    local -a ids descs
    local l
    _idea_lines ids -- "$1"
    for l in $reply; do
        ids+=("${l%%$'\t'*}")
        descs+=("${l%%$'\t'*}  ${l#*$'\t'}")
    done
    compadd -l -d descs -a ids
}

_idea_keyvalue() {
    local cur=$1
    if [[ $cur == *=* ]]; then
        local key=${cur%%=*}
        _idea_lines values "$key" -- "${cur#*=}"
        compadd -P "$key=" -a reply
    else
        _idea_lines keys -- "$cur"
        compadd -S = -a reply
    fi
}

_idea() {
    local cur=${words[CURRENT]}
    local -a reply
    if (( CURRENT == 2 )); then
        _idea_lines verbs -- "$cur"
        compadd -a reply
        return
    fi
    local verb=${words[2]} prev=${words[CURRENT-1]}

    case $prev in
        -t) _idea_lines tags -- "$cur"; compadd -a reply; return ;;
        -s) _idea_keyvalue "$cur"; return ;;
        --sort) compadd updated created reviewed title rank; return ;;
        --body-file|--old-file|--new-file) _files; return ;;
        --limit|--offset|--version) return ;;
    esac

    # pos is the position of the word under the cursor among the verb's
    # arguments, flags and their values left out.
    local pos=1 skip=0 t i
    for (( i = 3; i < CURRENT; i++ )); do
        t=${words[i]}
        if (( skip )); then
            skip=0
        else
            case $t in
                -t|-s|--body-file|--old-file|--new-file|--sort|--limit|--offset|--version) skip=1 ;;
                -?*) ;;
                *) pos=$((pos + 1)) ;;
            esac
        fi
    done

    case $verb in
        help) (( pos == 1 )) && { _idea_lines verbs -- "$cur"; compadd -a reply } ;;
        completion) (( pos == 1 )) && compadd fish zsh bash ;;
        attrs) (( pos == 1 )) && { _idea_lines keys -- "$cur"; compadd -a reply } ;;
        show|rm|reviewed) _idea_ids "$cur" ;;
        body|write|edit|replace|title) (( pos == 1 )) && _idea_ids "$cur" ;;
        tag|untag)
            if (( pos == 1 )); then _idea_ids "$cur"; else _idea_lines tags -- "$cur"; compadd -a reply; fi ;;
        set)
            if (( pos == 1 )); then _idea_ids "$cur"; else _idea_keyvalue "$cur"; fi ;;
        unset)
            if (( pos == 1 )); then _idea_ids "$cur"; else _idea_lines keys -- "$cur"; compadd -a reply; fi ;;
        status)
            if (( pos == 1 )); then _idea_ids "$cur"
            elif (( pos == 2 )); then _idea_lines values status -- "$cur"; compadd -a reply; fi ;;
    esac
}

if [[ $funcstack[1] == _idea ]]; then
    _idea "$@"
else
    compdef _idea idea
fi
