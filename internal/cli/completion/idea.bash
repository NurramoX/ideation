# bash completion for idea. Install with:
#   idea completion bash > ~/.local/share/bash-completion/completions/idea
# or add `source <(idea completion bash)` to ~/.bashrc.

# _idea_reply sets COMPREPLY from newline-separated candidates. Bash splits
# words at = and :, so the part of cur up to the last of them is dropped
# from each candidate.
_idea_reply() {
    local cur=$1 cands=$2 head c
    head=${cur%"${cur##*[=:]}"}
    local IFS=$'\n'
    for c in $cands; do
        COMPREPLY+=("${c#"$head"}")
    done
}

_idea_ids() {
    _idea_reply "$1" "$(idea complete ids -- "$1" | cut -f1)"
}

_idea_keyvalue() {
    local cur=$1
    if [[ $cur == *=* ]]; then
        local key=${cur%%=*}
        _idea_reply "$cur" "$(idea complete values "$key" -- "${cur#*=}" | sed "s/^/$key=/")"
    else
        _idea_reply "$cur" "$(idea complete keys -- "$cur" | sed 's/$/=/')"
        compopt -o nospace 2>/dev/null
    fi
}

_idea() {
    COMPREPLY=()
    # Words are split on spaces only, so key=value stays one word.
    local line=${COMP_LINE:0:COMP_POINT} cur
    local -a words
    read -ra words <<<"$line"
    if [[ $line == *' ' ]]; then
        cur=
    else
        cur=${words[${#words[@]}-1]}
        unset 'words[${#words[@]}-1]'
    fi
    if (( ${#words[@]} <= 1 )); then
        _idea_reply "$cur" "$(idea complete verbs -- "$cur")"
        return
    fi
    local verb=${words[1]} prev=${words[${#words[@]}-1]}

    case $prev in
        -t) _idea_reply "$cur" "$(idea complete tags -- "$cur")"; return ;;
        -s) _idea_keyvalue "$cur"; return ;;
        --sort) COMPREPLY=($(compgen -W 'updated created reviewed title rank' -- "$cur")); return ;;
        --body-file|--old-file|--new-file) COMPREPLY=($(compgen -f -- "$cur")); return ;;
        --limit|--offset|--version) return ;;
    esac

    # pos is the position of the word under the cursor among the verb's
    # arguments, flags and their values left out.
    local pos=1 skip=0 t i
    for (( i = 2; i < ${#words[@]}; i++ )); do
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
        help) (( pos == 1 )) && _idea_reply "$cur" "$(idea complete verbs -- "$cur")" ;;
        completion) (( pos == 1 )) && COMPREPLY=($(compgen -W 'fish zsh bash' -- "$cur")) ;;
        attrs) (( pos == 1 )) && _idea_reply "$cur" "$(idea complete keys -- "$cur")" ;;
        show|rm|reviewed) _idea_ids "$cur" ;;
        body|write|edit|replace|title) (( pos == 1 )) && _idea_ids "$cur" ;;
        tag|untag)
            if (( pos == 1 )); then _idea_ids "$cur"; else _idea_reply "$cur" "$(idea complete tags -- "$cur")"; fi ;;
        set)
            if (( pos == 1 )); then _idea_ids "$cur"; else _idea_keyvalue "$cur"; fi ;;
        unset)
            if (( pos == 1 )); then _idea_ids "$cur"; else _idea_reply "$cur" "$(idea complete keys -- "$cur")"; fi ;;
        status)
            if (( pos == 1 )); then _idea_ids "$cur"
            elif (( pos == 2 )); then _idea_reply "$cur" "$(idea complete values status -- "$cur")"; fi ;;
    esac
    return 0
}

complete -F _idea idea
