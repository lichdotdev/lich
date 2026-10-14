# Marks the prompt with OSC 133 so the terminal knows where it starts and
# ends, and asks it (redraw=1) to clear the prompt on a resize: zsh redraws
# the whole prompt on SIGWINCH, and without the clearing a prompt line the
# resize wrapped stays on screen above the one zsh draws again.
#
# The marks go around PS1 and never inside it: themes such as powerlevel10k
# keep newlines inside parameter expansions, which a mark there would break.
builtin typeset -gi _lich_ran=0

_lich_precmd() {
  builtin local -i st=$?
  (( _lich_ran )) && builtin print -n "\e]133;D;$st\a"
  _lich_ran=0
  # Themes rebuild PS1 in their own precmd; running last keeps the marks.
  if [[ ${precmd_functions[-1]} != _lich_precmd ]]; then
    precmd_functions=(${precmd_functions:#_lich_precmd} _lich_precmd)
  fi
  [[ $PS1 == *$'\e]133;A'* ]] || PS1=$'%{\e]133;A;redraw=1\a%}'"$PS1"$'%{\e]133;B\a%}'
}

_lich_preexec() {
  builtin print -n "\e]133;C\a"
  _lich_ran=1
}

builtin typeset -ga precmd_functions preexec_functions
precmd_functions+=(_lich_precmd)
preexec_functions+=(_lich_preexec)
