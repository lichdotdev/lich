## [0.69.0] - 2026-10-16

### Added

- **A janela nativa no macOS, experimental.** O Lich.app a traz junto com o lich
  no Apple Silicon e no Intel, e Configurações › Aparência › Janela a oferece como
  no Linux e no Windows. A tecla Option digita caracteres no terminal dela, como
  no Terminal.app, então atalhos Option+letra de um agente não chegam a ele. Até
  agora ela só rodou nas máquinas macOS do GitHub: as notificações são simples e
  não dá para clicar nelas, e não há item na barra de menus.
- **A árvore de arquivos reconhece muito mais tipos de arquivo.** Java, Gradle, C e C++,
  C#, PHP, Ruby, Swift, SQL, Vue, Svelte, Terraform, Dockerfile, .gitignore e
  mais umas trinta extensões agora mostram o próprio ícone na árvore de arquivos e
  nos resultados de busca, onde antes mostravam um glifo de arquivo genérico.

### Fixed

- **`lich quit` fecha a janela nativa.** Ela continuava aberta mostrando o lich como
  perdido depois que o backend se fora; o Encerrar o lich da bandeja e um sinal de parada
  também a fecham, enquanto um reinício depois de uma atualização ainda a mantém.
- **A janela nativa oferece remover o worktree quando a última sessão dele fecha.** O
  toast de sessão fechada ganha Remover worktree ao lado de Desfazer; um worktree que o lich não criou, ou
  um com alterações não commitadas, é citado em um diálogo antes de ser removido.

- **O terminal da janela nativa pega a célula sob o ponteiro.** Um clique,
  uma seleção por arrasto, um link e um programa que rastreia o mouse liam o
  ponteiro deslocado pelo padding do terminal, então a célula captada ficava abaixo
  e à direita da apontada, por quase uma célula inteira em uma tela
  escalada.
- **O terminal da janela nativa desenha emoji.** Uma fonte de emoji colorido como a
  Noto Color Emoji guarda cada emoji como imagem em vez de contorno, e o
  terminal só desenhava contornos, então todo emoji aparecia como um espaço vazio; agora
  eles são desenhados em cores, ocupando duas células.
- **A árvore de arquivos da janela nativa mostra o ícone de linguagem de cada arquivo.** Todo
  arquivo desenhava o mesmo glifo de arquivo genérico; cada um agora leva o ícone que a janela
  do navegador lhe dá.
- **Um pull request aberto por um agente aparece no card quando o turno termina.** O
  badge só consultava de novo quando a branch ou o último commit dela mudava, e abrir
  um pull request não muda nenhum dos dois, então ele esperava a janela perder e recuperar o
  foco.
- **A janela nativa abre no Windows.** Ela buscava as fontes pelo
  fontconfig, que o Windows não traz, então fechava na inicialização e o lich
  recorria à janela web. No Windows ela agora encontra as fontes instaladas
  por conta própria, Cascadia Mono ou Consolas para o terminal por padrão, e os
  seletores de fonte em Configurações as listam.
- **Alternar entre um tema claro e um escuro não trava mais a janela nativa.** Uma sessão que segue o esquema de cores do terminal, como o Claude Code
  faz, era avisada da mudança enquanto a janela ainda segurava seus terminais, então
  a janela travava e fechava.

