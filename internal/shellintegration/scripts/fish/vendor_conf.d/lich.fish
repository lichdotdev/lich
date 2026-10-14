# lich's fish integration, found through XDG_DATA_DIRS, which it gives back.
# fish marks its prompt with OSC 133 itself; this asks the terminal to clear
# the prompt on a resize (redraw=1), since fish redraws all of it.
if set -q LICH_SHELL_INTEGRATION_XDG_DIR
    if set -l index (contains -i -- $LICH_SHELL_INTEGRATION_XDG_DIR $XDG_DATA_DIRS)
        set -e XDG_DATA_DIRS[$index]
    end
    if test (count $XDG_DATA_DIRS) -eq 0
        set -e XDG_DATA_DIRS
    end
    set -e LICH_SHELL_INTEGRATION_XDG_DIR
end

status is-interactive; or return

function __lich_redraw_prompt --on-event fish_prompt
    printf '\e]133;A;redraw=1\a'
end
