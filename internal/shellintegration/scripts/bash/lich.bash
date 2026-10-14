# lich's bash integration, read through --rcfile in place of ~/.bashrc, which
# it reads first. Marks the prompt with OSC 133 so the terminal knows where
# it starts and ends, and asks it (redraw=last) to clear the prompt's last
# line on a resize, the only line readline draws again.
if [[ -r ~/.bashrc ]]; then
  builtin source ~/.bashrc
fi

_lich_prompt_marks() {
  builtin local st=$?
  builtin printf '\e]133;D;%s\a' "$st"
  [[ $PS1 == *'133;A'* ]] || PS1='\[\e]133;A;redraw=last\a\]'"$PS1"'\[\e]133;B\a\]'
  return "$st"
}

# Output starts once the command is read (bash 4.4+).
PS0+='\e]133;C\a'

if [[ "$(builtin declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
  PROMPT_COMMAND+=(_lich_prompt_marks)
else
  PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND$'\n'}_lich_prompt_marks"
fi
