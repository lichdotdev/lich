# lich's zsh integration starts here: ZDOTDIR points at this directory for
# the first file zsh reads, which hands ZDOTDIR back and reads the user's own
# .zshenv before loading the integration (the way Ghostty's does).
if [[ -n "${LICH_USER_ZDOTDIR+X}" ]]; then
  builtin export ZDOTDIR="$LICH_USER_ZDOTDIR"
  builtin unset LICH_USER_ZDOTDIR
else
  builtin unset ZDOTDIR
fi
{
  builtin typeset _lich_file="${ZDOTDIR-$HOME}/.zshenv"
  [[ ! -r "$_lich_file" ]] || builtin source -- "$_lich_file"
} always {
  if [[ -o interactive ]]; then
    builtin source -- "${${(%):-%x}:A:h}/lich-integration.zsh"
  fi
  builtin unset _lich_file
}
