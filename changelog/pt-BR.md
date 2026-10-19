## [0.71.2] - 2026-10-19

### Fixed

- A janela nativa volta a abrir no Windows e no macOS, onde fechava logo em
  seguida procurando o fontconfig para desenhar as visões de código e de diff.
- Emojis no terminal da janela nativa são desenhados em cores no Windows, onde
  apareciam só como contornos.
- O comando de atualização oferecido a uma instalação no macOS agora faz o tap de `lichdotdev/tap` antes de atualizar. Uma instalação vinda do antigo `omartelo/tap` falhava com "Cask 'lichdotdev/tap/lich' is unavailable", já que o Homebrew não segue um tap que mudou de lugar; faça o tap uma vez com `brew tap lichdotdev/tap` para atualizar a partir de um lich mais antigo.
- Na janela nativa, "Expandir pastes longos" agora expande os pastes do Claude Code e do Kiro CLI: a área de transferência, um lote de comentários de revisão, o prompt com que um pull request ou issue abre uma sessão e caminhos de arquivo soltos ou anexados.
