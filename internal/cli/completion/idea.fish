# fish completion for idea. Install with:
#   idea completion fish > ~/.config/fish/completions/idea.fish

# __idea_candidates prints the candidates for the token under the cursor.
function __idea_candidates
    set -l tokens (commandline -opc)
    set -l cur (commandline -ct)
    set -e tokens[1]
    if test (count $tokens) -eq 0
        idea complete verbs -- $cur
        return
    end
    set -l verb $tokens[1]
    set -e tokens[1]

    switch "$tokens[-1]"
        case -t
            idea complete tags -- $cur
            return
        case -s
            __idea_keyvalue $cur
            return
        case --sort
            printf '%s\n' updated created reviewed title rank
            return
        case --body-file --old-file --new-file
            __fish_complete_path $cur
            return
        case --limit --offset --version
            return
    end

    # pos is the position of the token under the cursor among the verb's
    # arguments, flags and their values left out.
    set -l pos 1
    set -l skip 0
    for t in $tokens
        if test $skip -eq 1
            set skip 0
        else if contains -- $t -t -s --body-file --old-file --new-file --sort --limit --offset --version
            set skip 1
        else if not string match -q -- '-?*' $t
            set pos (math $pos + 1)
        end
    end

    switch $verb
        case help
            test $pos -eq 1; and idea complete verbs -- $cur
        case completion
            test $pos -eq 1; and printf '%s\n' fish zsh bash
        case attrs
            test $pos -eq 1; and idea complete keys -- $cur
        case show rm reviewed
            idea complete ids -- $cur
        case body write edit replace title
            test $pos -eq 1; and idea complete ids -- $cur
        case tag untag
            if test $pos -eq 1
                idea complete ids -- $cur
            else
                idea complete tags -- $cur
            end
        case set
            if test $pos -eq 1
                idea complete ids -- $cur
            else
                __idea_keyvalue $cur
            end
        case unset
            if test $pos -eq 1
                idea complete ids -- $cur
            else
                idea complete keys -- $cur
            end
        case status
            switch $pos
                case 1
                    idea complete ids -- $cur
                case 2
                    idea complete values status -- $cur
            end
    end
end

# __idea_keyvalue completes key=, then the key's values.
function __idea_keyvalue -a cur
    if string match -q -- '*=*' $cur
        set -l kv (string split -m 1 = -- $cur)
        idea complete values $kv[1] -- $kv[2] | string replace -r -- '^' "$kv[1]="
    else
        idea complete keys -- $cur | string replace -r '$' =
    end
end

complete -c idea -f -a '(__idea_candidates)'
