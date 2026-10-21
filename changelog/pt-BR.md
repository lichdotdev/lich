## [0.75.0] - 2026-10-21

### Added

- Um arquivo Markdown na aba Revisão tem um botão de prévia ao lado de "Adicionar arquivo como contexto" que mostra o arquivo renderizado, com tabelas e links, no lugar do diff. Clique de novo para voltar ao diff. Na tela Pull requests, só os arquivos que o pull request adiciona têm o botão.
- Ctrl+F (Cmd+F no macOS) em um arquivo aberto na aba Código procura texto nele. Uma barra sob o nome do arquivo destaca cada ocorrência, percorre-as com Enter e Shift+Enter e pode diferenciar maiúsculas de minúsculas, buscar palavras inteiras ou usar uma expressão regular. O botão de lista dela mostra sob a barra cada linha com uma ocorrência, e clicar em uma leva o código até lá. Escape a fecha.
- O tooltip do card de uma sessão informa o modelo que a sessão está rodando, no Claude Code, no Codex e no Kiro CLI. Quando já não é o modelo com que a sessão foi aberta, depois de um `/model`, por exemplo, a linha diz "Aberta com …, rodando …".
- A tela Pull requests edita o título e a descrição de um pull request e os envia ao GitHub. O lápis depois do título o renomeia ali mesmo: Enter ou clicar fora salva, Escape cancela. Editar ao lado da Descrição da Visão geral a abre como Markdown, salva com Salvar ou Ctrl+Enter (Cmd+Enter no macOS). Uma descrição longa rola dentro do campo, com barra de rolagem, para que Salvar continue na tela. Cada uma envia só o que mudou, então salvar a descrição nunca desfaz uma renomeação feita no GitHub nesse meio tempo.
- Um perfil pode ter cor e imagem próprias. Em Configurações › Provedores, o menu ⋯ de um perfil tem Cor, com as mesmas oito cores de um card de sessão, e Escolher imagem…, que aceita um PNG, JPEG, GIF ou WebP de até 10 MB e mantém o quadrado central dele. A imagem ou a cor aparece onde quer que o perfil apareça: a lista e o seletor de perfis, o rodapé da barra lateral e Continuar em. Remover imagem volta para a inicial na cor do perfil.

### Changed

- Um bloco de cards na barra lateral, uma worktree, uma pasta ou o bloco fixado, agora cresce quando o primeiro card abre e encolhe com o último, onde antes aparecia e sumia de uma vez. Os cabeçalhos que aparecem quando a lista se divide em blocos se abrem do mesmo jeito e se recolhem quando sobra um só.
- As verificações de um pull request ainda em execução agora mostram um spinner girando, na aba Verificações e na linha de status acima das abas, onde antes havia um relógio parado.
- Os botões não afundam mais um pixel enquanto são pressionados, então todos os botões da janela se comportam do mesmo jeito: os que tinham esse movimento agora combinam com os que nunca tiveram.

### Removed

- **Breaking:** o lich não executa mais o `.lich/setup-worktree.sh` quando `lich open --worktree` ou o `open_session` de um agente inicia uma sessão em uma worktree nova. O arquivo no seu repositório não é tocado. Se as suas worktrees dependiam dele para instalar dependências, execute-o você mesmo no terminal da nova worktree (`sh .lich/setup-worktree.sh`), ou peça ao agente. A cópia dos seus arquivos `.env*` ignorados pelo git (ou do que o `.worktreeinclude` nomeia) e a porta de servidor de desenvolvimento `LICH_WORKTREE_PORT` continuam iguais.

### Fixed

- O `git commit`, `rebase` ou `stash` de um agente não falha mais de vez em quando com "Unable to create '.git/index.lock': File exists" porque o lich estava lendo o status ou o diff daquele checkout no mesmo instante. As leituras do lich deixam o índice em paz.
- O card de uma sessão fechada volta a encolher para fora da barra lateral, onde antes sumia de uma vez.
- Ctrl+Z em uma sessão não congela mais o agente para sempre. Antes ele suspendia o agente, e nada no lich conseguia trazê-lo de volta; agora o lich mantém a tecla longe da sessão.
- As barras do tooltip de uso do plano, no rodapé, voltam a mostrar a trilha vazia em temas cujas cores de popover e muted são iguais, como o Everforest, onde só a parte preenchida era desenhada.
- Abrir ou fechar a lista de pull requests na tela Pull requests não faz mais a aba do projeto ativo soltar o painel abaixo dela e agarrá-lo de novo. A aba continua conectada durante todo o movimento e passa para a lista quando ela desliza por baixo.
