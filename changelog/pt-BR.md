## [0.71.0] - 2026-10-17

### Added

- **Layouts divididos na janela nativa (experimental).** Coloque sessões de
  qualquer projeto aberto lado a lado e divida-as como quiser: Ctrl+Shift+G
  divide a sessão em foco para a direita, Ctrl+Shift+I para baixo, e cada um
  pergunta qual sessão vai ali; ou arraste um card da barra lateral até a borda
  de um painel. Reorganize um layout arrastando um painel pelo cabeçalho até a
  borda de outro painel, ou até o meio dele para trocar os dois de lugar, e
  alterne um par entre lado a lado e empilhado com Ctrl+Shift+R. Cada layout é
  uma aba depois dos projetos, com o status dos seus painéis, e é renomeado com
  um clique duplo; arraste o vão entre dois painéis para redimensioná-los,
  amplie um com Ctrl+Shift+M, alterne entre eles com Ctrl+Shift+F e tire um do
  layout com Ctrl+Shift+W; um layout reduzido a um painel é fechado. Deixe uma
  sessão arrastada sobre outra aba e essa aba abre, como um navegador faz com um
  arquivo arrastado. A barra lateral e o rodapé acompanham o painel em foco.
- **Mostre os delegados de uma sessão ao lado dela.** Na janela nativa, uma
  sessão que abriu outras, em qualquer projeto, oferece essas sessões no
  rodapé: Mostrar ao lado organiza a sessão com os delegados empilhados à
  direita, quantos couberem. Em um layout, o cabeçalho do painel conta quantos
  estão ali e adiciona os restantes, perguntando antes de tirar algum de outro
  layout. Um card em um layout o indica pelo nome.
- **Verificações obrigatórias têm rótulo.** Na janela nativa, a aba
  Verificações de um pull request marca com Obrigatório as verificações que
  condicionam o merge, como faz o GitHub.

### Changed

- **O lich agora fica em `lichdotdev/lich`.** O repositório foi movido para a
  organização [lichdotdev](https://github.com/lichdotdev) e o site para
  [lichdotdev.github.io/lich](https://lichdotdev.github.io/lich/). Links
  antigos do GitHub, clones e verificações de atualização são redirecionados;
  favoritos do site antigo não.

### Fixed

- **Novidades acompanha o idioma para o qual você troca.** Na janela nativa, as
  notas da versão continuavam no idioma em que foram lidas pela primeira vez
  depois que você mudava o idioma da interface, sob nomes de grupo no idioma
  novo.
- **Diffs não travam mais com texto CJK nem acumulam memória por arquivo.** Na
  janela nativa, uma Revisão ou os Arquivos de um pull request com texto em
  chinês, japonês ou coreano podia congelar por segundos enquanto cada
  caractere novo era buscado, e o diff de cada arquivo mantinha a sua própria
  cópia das fontes, de modo que um pull request grande ocupava centenas de
  megabytes.
