## [0.78.1] - 2026-10-23

### Changed

- Enquanto uma sessão trabalha, a janela redesenha o spinner de ocupado a 30 quadros por segundo, e não na taxa da tela; o card monta o menu de clique direito só enquanto ele está aberto, e uma caixa de diálogo fechada não monta nada.

### Fixed

- Branch, alterações e pull request nos cards e no rodapé se atualizam em até um segundo depois de um commit, checkout ou edição feitos fora de um turno, onde antes podiam atrasar até três segundos, e selecionar um card lê o checkout dele na hora, em vez de mostrar o que a última leitura encontrou.
- Ocultar a lista de pull requests na tela Pull requests a recolhe na hora, onde a lista ficava na tela até o mouse se mover ou a tela Pull requests consultar o GitHub de novo. Mostrá-la outra vez também não espera mais um quadro.
- As sessões do Kiro CLI seguem `KIRO_HOME`. Com ela definida, o lich lia as conversas, o custo e o contexto do Kiro em `~/.kiro`, enquanto o Kiro os mantinha em `KIRO_HOME`, então nada aparecia e o Histórico não listava nenhuma; o agente que carrega os hooks do lich era instalado onde o Kiro nunca olhava, e uma sessão em sandbox não recebia o diretório em que ele escreve.
- Abrir ou fechar o dock não deixa mais o terminal da sessão cortado na borda por um instante depois que o dock para. O terminal assume a nova largura assim que o dock começa a se mover, então o agente redesenha enquanto o dock desliza, onde o terminal esperava um décimo de segundo depois que o dock parava e então dava um salto. Os terminais de um layout dividido se ajustam no instante em que o dock para.
- Expandir a barra lateral não deixa mais o terminal da sessão na largura antiga por um instante depois que a barra se abre. O terminal assume a nova largura assim que a barra começa a se mover, como no dock, onde esperava um décimo de segundo depois que a barra parava e então dava um salto; recolhê-la também reajusta na hora. Os terminais de um layout dividido se ajustam no instante em que a barra lateral para.
- Reabrir a janela não digita mais texto solto em uma sessão. Reconstruir um terminal a partir da saída recente respondia de novo às consultas do terminal nela, como as que o nvim envia ao iniciar, e as respostas caíam na entrada da sessão: em um shell como um `command not found`, em um agente como texto no prompt dele.
- O terminal de uma sessão aberto enquanto ela imprime não perde mais o que foi impresso nesse momento. O backend agora entrega à janela a saída recente da sessão em linha com a saída ao vivo, então nada entre as duas se perde nem aparece duas vezes.
- No Linux, mudar o tamanho da fonte do terminal e abrir um card pela primeira vez travam menos. Cada terminal consultava a fonte de novo pelo fontconfig a cada vez, quatro consultas de cerca de 10 ms cada uma dentro de um quadro; o lich agora lembra a resposta pelo resto da execução.
- Mudar o tamanho da fonte do terminal não recarrega mais a fonte de todos os terminais abertos: cada um mantém a fonte que tem e só a desenha no novo tamanho.
- Uma sessão que imprime muita saída custa menos ao backend. Cada byte a caminho da janela era copiado cerca de sete vezes para uma memória que o coletor de lixo depois precisava recuperar; agora é copiado uma vez.
- Um comentário de bloco que abre acima de um trecho recolhido de um diff, na Revisão e na tela Pull requests, não pinta mais de comentário o código depois desse trecho. O `*/` que o fechava ficava nas linhas ocultas, então todo hunk abaixo aparecia como comentário até algum `*/` mais adiante, e só expandir o trecho resolvia. As linhas de um comentário cortado por um trecho recolhido, acima dele, ainda podem aparecer como código até que o trecho seja expandido.

## [0.78.0] - 2026-10-22

### Added

- Comentários podem ser editados e excluídos na tela Pull requests. O … ao lado do comentário de uma thread de revisão (na aba Conversa e no diff) ou de um comentário do pull request oferece Editar, que o reescreve ali mesmo no GitHub, e Excluir, que pergunta antes. O resumo de uma revisão enviada também pode ser editado; o veredito dela continua como está. O … aparece só no que o GitHub permite que a sua conta altere.
- Escolher uma fonte monoespaçada para a interface em Configurações › Aparência pergunta se o terminal e o editor também devem usá-la. Usar em tudo define as duas; Só a interface as mantém como estavam. Uma fonte proporcional ou Sistema não pergunta nada.
- Um comentário que espera no lote de revisão pode ser reescrito antes de ser enviado: o lápis dele o abre em uma caixa de diálogo, salvo com Salvar ou Ctrl+Enter. Um comentário de linha na revisão pendente de um pull request tem um botão Editar ao lado de Descartar que o abre em uma caixa, salvo com Salvar ou Ctrl+Enter.

### Removed

- O aviso de que a janela nativa é experimental, mostrado na primeira vez que ela abria, saiu: a janela nativa é a única janela do lich.

### Fixed

- Os comentários para a sessão escritos na aba Arquivos de um pull request na tela Pull requests vão para uma sessão no checkout da branch desse pull request, onde Enviar os digitava na sessão que estivesse aberta antes da tela Pull requests, revisando outra branch. Sem uma sessão ali, Enviar diz para abrir uma e mantém os comentários; Abrir em sessão a inicia. Uma referência de linha enviada dessa aba segue o mesmo caminho.
- Ir para a sessão em um pull request na tela Pull requests abre a sessão em que você está quando ela fica no checkout do pull request, onde sempre abria a primeira sessão ali.
- No Windows com escala de tela acima de 100%, o arrastar volta a acompanhar o ponteiro: um item arrastado em Configurações › Rodapé ou uma seleção arrastada no terminal ficavam longe do cursor, escalados como se a tela estivesse em 100%.
- Os comentários de revisão enviados a uma sessão são mantidos até a sessão recebê-los. Uma sessão que recusou o lote (uma que tinha parado, por exemplo) mostrava "Não foi possível enviar os comentários" enquanto os comentários já tinham sumido; agora eles continuam no lote para serem enviados de novo. Da mesma forma, enviar a revisão de um pull request na tela Pull requests descarta só os comentários que ela levou, onde um comentário escrito enquanto a revisão estava a caminho se perdia com eles.
- No Windows, a janela usa o ícone do lich e abre sob o botão do lich na barra de tarefas, onde mostrava um ícone genérico e, aberta a partir de um lich fixado na barra de tarefas, abria como um botão separado ao lado do fixado.
- Cliques em toda a janela mostram o resultado na hora, onde muitos esperavam o mouse se mover: caixas de diálogo abrindo e fechando, os botões de fechar do dock e das Configurações, a abertura de um arquivo a partir da aba Código, as opções da barra de busca, os filtros e a conversa da tela Pull requests, o filtro da barra lateral e as seções, os switches, as escadas e os seletores das Configurações. Um controle segmentado não mostra mais duas opções pressionadas logo após um clique.
- Um clique em uma pasta na árvore de arquivos da aba Código, ou na árvore de arquivos de um pull request, a abre ou a recolhe na hora, onde esperava o mouse se mover. A aba Código também monta em milissegundos a árvore de um checkout com dezenas de milhares de arquivos, onde levava segundos, e o hover de uma linha acompanha o ponteiro sem ficar um evento atrasado.
- Reiniciar o lich após uma atualização abre a janela da nova versão, onde a janela antiga continuava aberta sobre o novo lich e uma sessão podia falhar ao iniciar com "want 8 arguments, got 9" até o lich ser fechado e aberto de novo. A janela fecha e volta com a reinicialização; um lich rodando sem janela continua sem uma. Isso vale a partir da próxima atualização depois desta versão.
- Ctrl+F em um arquivo aberto na aba Código abre a barra de busca, e as teclas digitadas ali chegam a ela, onde ambos iam para o terminal da sessão atrás da aba: o terminal tomava o teclado de volta do código, da barra de busca e da lista Mostrar todas as ocorrências a cada quadro. Um clique no terminal agora lhe entrega o teclado, então selecionar código para copiar não deixa mais você digitando no nada.
- Um pull request na lista da tela Pull requests não corta mais o veredito da revisão quando o nome do autor é longo: o nome encurta, e a pasta do checkout no fim da linha aparece só quando cabe inteira, em vez de meio ícone.
- Um pull request na lista da tela Pull requests cujas threads de revisão estão todas resolvidas mostra "Alterações solicitadas" em âmbar com uma marca de resolvido, como o título dele já fazia, em vez do vermelho de quem ainda espera o autor. O GitHub mantém o veredito até o revisor aprovar ou a revisão ser descartada. O "Rascunho" de um rascunho agora é um selo cinza com contorno, para não parecer parte de um veredito âmbar ao lado.

## [0.77.0] - 2026-10-21

### Added

- A tela Pull requests mostra uma imagem alterada na aba Arquivos de um pull request, com o antes e o depois lado a lado, como a aba Revisão faz, onde antes dizia apenas "Arquivo binário". Um arquivo Markdown editado ali também tem o botão de prévia, onde só um arquivo adicionado o tinha. O lich lê o arquivo do seu clone quando ele tem o commit e do GitHub caso contrário.
- O `lich mcp` responde a clientes MCP na revisão de protocolo 2026-07-28, que informam a versão em cada requisição em vez de abrir com `initialize`, e atende o `server/discover` deles. Clientes que abrem com `initialize` são atendidos exatamente como antes.
- O card de uma sessão do Cursor CLI assume o nome que o Cursor dá à conversa, quando o primeiro turno termina, onde antes mantinha o nome dado pelo lich.

### Changed

- Um merge na tela Pull requests avisa também a sessão que abriu a sessão da branch mesclada, onde quer que ela rode, como já avisava as sessões do checkout mesclado. Uma sessão que distribui trabalho fica sabendo que o pull request dela foi mesclado sem você precisar dizer.

### Fixed

- Resolver conflitos, Corrigir erros de CI e uma worktree iniciada a partir de uma issue enviam a mensagem a uma nova sessão do Claude Code assim que ela está no prompt, onde antes ela podia ficar sem ser enviada por até um minuto, até você apertar Enter. O lich lia as respostas do próprio terminal às perguntas de inicialização do Claude Code como você digitando naquele prompt, e segurava a mensagem para você.
- Um arquivo Markdown longo, como o próprio CHANGELOG.md do lich, tem prévia nas abas Código e Revisão sem congelar a janela: a prévia desenha só a parte visível, onde antes calculava o layout do arquivo inteiro a cada quadro.
- Uma prévia de Markdown mantém as palavras separadas onde o código-fonte quebra uma linha, onde antes elas se juntavam, e uma tabela não espreme mais uma coluna estreita em palavras quebradas: cada coluna tem pelo menos a largura da sua palavra mais longa.
- O card do Claude Code volta a assumir o título que o Claude Code dá à conversa, onde antes mantinha o nome dado pelo lich. O lich agora inicia uma nova sessão do Claude Code sem `--name`, o que impedia o Claude Code de dar título a ela. A sessão escolhe o próprio nome na lista de agentes do Claude Code, diferente do do card, e `lich sessions`, `lich whoami` e `list_sessions` não mostram um segundo nome para ela até um `/rename` dar um.

## [0.76.0] - 2026-10-21

### Added

- Um arquivo Markdown aberto na aba Código tem o mesmo botão de prévia da aba Revisão, ao lado da marca de somente leitura, e mostra o arquivo renderizado, com tabelas e links, no lugar do código-fonte. Clique de novo para voltar ao código-fonte. Qualquer outro arquivo abre como código-fonte.

### Changed

- Enviar em um lote de comentários de revisão, nas abas Revisão e Código e na tela Pull requests, envia os comentários à sessão como o prompt dela, onde antes ficavam no prompt esperando um Enter. Uma sessão de shell mantém os comentários, já que ali um Enter os executaria como comando.

### Fixed

- Teclas mortas compõem no terminal de uma sessão: no macOS, ~ e depois a digita ã, e ' e depois c digita ç, onde antes o acento chegava sozinho antes da letra. O texto que um método de entrada ainda está compondo, como o de um IME japonês ou chinês, também não chega mais ao programa antes de ser confirmado.
- Depois de trocar de modelo em uma sessão do Claude Code com `/model`, o rodapé e o tooltip do card da sessão mostram o novo modelo assim que o primeiro turno dele termina. Antes mantinham o antigo até um turno posterior ou uma reconexão, porque o lich lia o transcript antes de o Claude Code ter escrito a última linha daquele turno.
- No macOS, os botões de fechar, minimizar e zoom continuam centralizados na barra de título. O macOS os devolvia ao topo da barra quando a janela abria, quando você trocava para outro app e voltava, e quando o sistema alternava entre claro e escuro, até a janela ser redimensionada.

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

## [0.74.0] - 2026-10-20

### Added

- F11 coloca a janela em tela cheia e a tira de lá, sem os botões de minimizar, maximizar e fechar enquanto ela está assim. O atalho pode ser alterado em Configurações › Atalhos.

### Changed

- Um pull request que conflita com a base não lista mais os arquivos em conflito sob o título. O status "Conflita com" os conta, e clicar nele abre um painel sobre a tela que nomeia cada um.
- **Resolver conflitos** e **Corrigir erros de CI** em um pull request, e uma worktree iniciada a partir de uma issue `#123`, agora enviam a mensagem para a sessão que abrem, onde antes ela ficava no prompt esperando você apertar Enter. Um card de terminal nesse checkout nunca recebe nada.

### Fixed

- A aba do projeto ativo agora se separa da ilha do terminal quando abrir ou redimensionar o painel Código ou Revisão faz com que ela invada o painel, e se reconecta quando volta a caber.
- No macOS, os botões de fechar, minimizar e zoom ficam centralizados na barra de título da janela nativa, e não colados na borda superior, e acompanham o zoom da interface.
- A janela pode ser arrastada pela barra superior: depois da última aba e nos espaços ao redor da marca e dos botões. Antes, nenhuma parte da barra a movia.
- A tela Pull requests mantém o que mostra atualizado sem apertar Atualizar. Ela lê um pull request de novo a cada poucos segundos enquanto o GitHub ainda decide se ele pode ser mergeado, uma vez por minuto enquanto a tela está aberta e quando você volta para a janela, então um push, um merge ou um conflito feito fora do lich aparece sozinho.

## [0.73.0] - 2026-10-20

> [!IMPORTANT]
> **Todos: a janela nativa agora é a única janela do lich.** O lich a abre em toda
> inicialização; a janela web e o Chromium embutido acabaram.

### Added

- **Notificações de desktop em que você pode clicar, na janela nativa no Windows e no macOS.** Como no Linux, uma sessão que precisa da sua resposta ou terminou o turno gera uma notificação com Abrir e Dispensar: Abrir leva o lich ao card dessa sessão, e a notificação some quando você responde ou abre a sessão. No macOS ela exige que o lich rode a partir do Lich.app e pede permissão uma vez; no Windows, uma sessão esperando por você mantém a notificação na tela até você agir sobre ela.
- Configurações › Provedores avisa em Perfil quando as sessões iniciam por um binário personalizado, com o caminho dele e botões para mostrá-lo ou voltar ao padrão. Um wrapper que define o próprio diretório de login ou de configuração sobrepõe o perfil, e a configuração Binário fica recolhida no fim da tela.

### Changed

- Quando o processo de uma sessão termina, um toast no canto inferior direito agora pergunta o que fazer, no lugar da faixa na parte de baixo do terminal. Um card com conversa oferece Retomar, que abre de novo essa mesma conversa (no perfil em que ela começou), Começar outra, que inicia uma nova no card como o Reiniciar fazia, e Fechar sessão. Um shell, ou um agente que terminou antes do primeiro prompt, oferece Reiniciar e Fechar sessão. Dispensar o toast deixa o card como está, e focar o card traz o toast de volta.
- **O perfil do projeto fica no pé da barra lateral, não em cada card.** Com mais de um login para um provedor, a última linha da barra lateral mostra o perfil em que as novas sessões desse provedor neste projeto começam: o provedor do card na tela, ou o padrão para um terminal. Clique nela para escolher outro login, com o uso de cinco horas de cada um, ou para voltar ao Global; Outros provedores altera os demais pelo mesmo menu. Os cards não trazem mais o chip de perfil ao lado da branch.

### Removed

- A janela web. O lich abre a janela nativa, e uma release não inclui mais o Chromium. A escolha Janela nas Configurações (web ou nativa) e o item da bandeja que abria a outra janela acabaram, assim como `lich -- <chromium flags>`, `lich --shell` e `LICH_SHELL`. Se a janela nativa não consegue iniciar, o lich diz o motivo e onde está o log.

### Fixed

- Fazer login de um perfil do Claude Code ou do Codex agora mostra que está autenticando assim que você termina no navegador, ou envia um código colado, onde o diálogo ficava esperando o login até o fim e o campo do código apenas esvaziava.
- Em português do Brasil, toda mensagem agora chama a worktree de "a worktree", onde algumas diziam "o worktree".
- No macOS a janela nativa voltou a ter os botões de fechar, minimizar e zoom; desde a 0.72.0 a barra superior deixava o espaço deles vazio.
- No Windows, atualizar o lich pela janela nativa não faz mais rollback com "Setup was unable to automatically close all applications": a janela ficava aberta em `lich-native.exe`, que o instalador precisa substituir. Agora ela fecha antes de o instalador rodar e abre de novo quando o lich volta. Atualizar da 0.72.0 ou anterior ainda cai nisso uma vez: encerre `lich-native.exe` no Gerenciador de Tarefas quando a mensagem aparecer, escolha Retry e abra o lich pelo menu Iniciar depois.
- Fechar a janela nativa enquanto o lich está ausente, depois de ele encerrar ou enquanto reinicia, a fecha, seja qual for o valor de "Ao fechar a janela". Com Perguntar ou Encerrar ela ficava aberta e só mostrava um erro de que as sessões não puderam ser contadas.
- Numa janela de Configurações estreita, uma configuração cujo controle é largo demais para dividir a linha, como o seletor de Perfil de um provedor, agora põe o controle abaixo do título e da descrição, onde o texto era espremido numa faixa mínima e quebrava letra por letra. O link de Documentação de um provedor também passa para baixo do nome, em vez de bater no interruptor Ativado.
- Continuar em pode mover uma conversa do Claude Code de volta ao perfil que ela acabou de deixar, onde um hook de plugin que continuou rodando depois do fim da sessão (o do WakaTime, por exemplo) deixava um arquivo para trás e a movimentação era recusada com "already holds".

## [0.72.0] - 2026-10-19

### Changed

- A barra superior da janela nativa agora é a barra de título: a barra do sistema sumiu, e minimizar, maximizar e fechar ficam ao lado de Configurações, desenhados como o Windows, o GNOME e o KDE os desenham e no lado em que o seu desktop os coloca; no macOS os semáforos ficam na barra superior. Arraste a parte vazia da barra superior para mover a janela e dê um duplo clique nela para maximizar. Gerenciadores de janela tiling mantêm as próprias decorações.

### Fixed

- `lich quit` não falha mais com "EOF" quando o lich fecha antes de responder: ele espera a porta fechar em vez de tratar a resposta perdida como recusa.
- No Wayland, a janela nativa mostra o cursor do próprio desktop, no tema e no tamanho dele, onde podia mostrar outro tema. Num monitor com escala menor que a do maior, o cursor não pula mais ao entrar na janela.
- No Linux, a janela nativa se agrupa sob o ícone do próprio lich no dock, onde aparecia como um app separado com um ícone genérico.
- Na Revisão da janela nativa e nos Arquivos de um pull request, a barra de rolagem mantém o tamanho enquanto você rola, onde crescia e encolhia conforme o arquivo que ocupava a tela. Arrastá-la agora percorre o diff por igual, não arquivo por arquivo.

## [0.71.2] - 2026-10-19

### Fixed

- A janela nativa volta a abrir no Windows e no macOS, onde fechava logo em
  seguida procurando o fontconfig para desenhar as visões de código e de diff.
- Emojis no terminal da janela nativa são desenhados em cores no Windows, onde
  apareciam só como contornos.
- O comando de atualização oferecido a uma instalação no macOS agora faz o tap de `lichdotdev/tap` antes de atualizar. Uma instalação vinda do antigo `omartelo/tap` falhava com "Cask 'lichdotdev/tap/lich' is unavailable", já que o Homebrew não segue um tap que mudou de lugar; faça o tap uma vez com `brew tap lichdotdev/tap` para atualizar a partir de um lich mais antigo.
- Na janela nativa, "Expandir pastes longos" agora expande os pastes do Claude Code e do Kiro CLI: a área de transferência, um lote de comentários de revisão, o prompt com que um pull request ou issue abre uma sessão e caminhos de arquivo soltos ou anexados.

## [0.71.1] - 2026-10-18

### Fixed

- A janela nativa no Linux não trava mais com uma instrução ilegal ao abrir um terminal numa CPU sem AVX-512.

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

## [0.70.0] - 2026-10-17

### Added

- **O Novidades fala a sua língua.** O diálogo que abre depois de uma atualização
  mostra as notas da versão, e os nomes dos grupos delas, no idioma da interface
  que você escolheu, nas duas janelas. Uma versão cujas notas ainda não têm
  tradução as mostra em inglês.
- **Fork de uma sessão na janela nativa.** Clique com o botão direito em um card e
  escolha Fork › Aqui para continuar a conversa dele em um novo card logo abaixo,
  no mesmo checkout, ou Fork › Worktree… para levá-la a uma nova worktree junto
  com as suas alterações não commitadas. A cópia recebe o nome do original
  ("auth-refactor 2") e leva uma marca de fork. Só Claude Code, Codex e opencode:
  os outros provedores só retomam.
- **A janela nativa avisa quando dois cards rodam uma só conversa.** Retomar, de
  dentro da TUI do próprio agente, uma conversa que outro card já tem aberta
  mostra um aviso no canto inferior direito; Trocar fecha o card que retomou e
  traz à frente o que já a tinha.
- **A janela nativa mostra imagens.** Um PNG, JPEG, GIF, WebP ou BMP abre na
  prévia da aba Arquivos sobre um xadrez, com seu tamanho e dimensões, e uma
  imagem alterada na Revisão aparece antes e depois lado a lado. Um PDF, ou
  qualquer outro arquivo sem prévia, oferece Abrir no app padrão e Mostrar na
  pasta em vez de um erro; em um pull request, um arquivo binário leva um link
  para ele mesmo no GitHub.
- **A janela nativa no seu idioma.** Todas as telas da janela nativa aparecem em
  português, espanhol e chinês, e Configurações › Geral tem o idioma da interface
  e o idioma dos prompts, como a janela web os tem em Aparência. Antes de você
  escolher um, a interface segue o idioma do seu sistema. Escolher chinês em uma
  máquina sem fonte chinesa oferece baixar a Noto Sans SC (cerca de 25 MB) para a
  pasta própria do lich.
- **Japonês.** As duas janelas aparecem em japonês, e o japonês é um idioma de
  prompt, então o lich fala com os seus agentes nele. A janela nativa desenha
  japonês, chinês e coreano cada um na fonte da sua própria região quando há uma
  instalada (Noto Sans CJK, Yu Gothic, Hiragino, Malgun Gothic e semelhantes),
  então os kanji não tomam mais formas chinesas, e oferece baixar a Noto Sans JP
  ou KR quando não há, como faz com a Noto Sans SC para o chinês. O README também
  está em japonês.
- **Coreano.** As duas janelas aparecem em coreano, escolhido em Configurações ›
  Aparência na web e em Configurações › Geral na janela nativa, e o coreano é um
  idioma de prompt: briefings, mensagens repassadas e avisos chegam aos seus
  agentes em coreano.
- **O lich fala russo.** Escolha Русский como idioma da interface, na janela web
  em Configurações › Aparência ou na janela nativa em Configurações › Geral, e
  como idioma dos prompts, para que o briefing, as notas de relay e os handoffs
  que o lich digita nos seus agentes também apareçam em russo. As contagens
  seguem as formas um, poucos e muitos do russo (1 файл, 2 файла, 5 файлов).
- **Perfis do opencode no macOS e no Windows, e perfis do Cursor CLI no
  Windows.** Configurações › Provedores na janela nativa os lista como faz no
  Linux. O Cursor CLI no macOS e o Kiro CLI no macOS e no Windows continuam sem
  eles: ali guardam o login no Keychain ou em uma pasta que nenhuma configuração
  consegue mover, então todo perfil compartilharia uma só conta.

### Changed

- **Toda atualização abre a janela nativa.** A primeira execução de cada versão
  muda o lich para a janela nativa, recorrendo à web onde ela não consegue
  iniciar; volte nas Configurações e o lich fica na janela web até a próxima
  versão. Na primeira vez que a janela nativa abre, ela diz que é experimental e
  leva a um relato de bug.
- **Um anel de status mais leve na janela nativa.** O anel em volta do ícone do
  card de uma sessão é desenhado mais fino, para ter o peso que a janela web lhe
  dá.

- **Teclas de atalho mais folgadas na janela nativa.** As teclas na paleta de
  comandos, na janela de atalhos e em Configurações › Atalhos têm mais espaço em
  volta do texto, que fica centralizado.

- **As Configurações da janela nativa abrem em Global.** A engrenagem na barra
  superior sempre abre as Configurações em Global, em vez do escopo em que você
  as deixou por último; a aba de um projeto continua abrindo as configurações
  dele em Configurações do projeto….

- **Uma paleta de comandos mais calma na janela nativa.** Cada sessão, projeto e
  mensagem ocupa uma linha, com o caminho depois do nome; o caminho de uma
  worktree mostra as suas três últimas partes. As sessões que esperam por você
  vêm primeiro e dizem isso, e as abas de filtro trocam as pílulas por um
  sublinhado. Uma sessão fechada no Histórico mostra a branch e a idade à
  direita, e uma segunda linha só quando uma busca encontrou a conversa dela,
  com as palavras marcadas.

### Fixed

- **Uma sessão reiniciada não aparece mais como ocupada para sempre.** Uma sessão
  cujo agente morreu no meio de um turno, ou que foi fechada no meio de um turno,
  voltava no mesmo card ainda listada como ocupada para as outras sessões: uma
  tarefa enviada a ela esperava por um turno que nunca terminaria. Agora ela
  começa sem estado, como uma nova.
- **Codex, Cursor CLI, Antigravity e Crush deixam de perguntar o que você já
  respondeu.** Uma sessão em sandbox de qualquer um deles abre direto no prompt.
  O Cursor CLI e o Antigravity não perguntam mais de novo em uma worktree de uma
  pasta em que você confiou, nem o Antigravity nas subpastas dela, e o Crush não
  pede mais para inicializar cada nova worktree. Uma sessão do oh-my-pi em
  sandbox não abre mais no assistente de configuração.
- **O Claude Code deixa de perguntar "confiar nesta pasta?" onde você já
  respondeu.** Uma pasta em que você confiou na sua conta principal passa a ser
  confiável em todos os outros perfis do Claude Code a partir da próxima sessão
  ali, e uma sessão do Claude Code em sandbox abre sem a pergunta. Uma sessão que
  o lich abriu para você não fica mais parada nessa tela, onde Enter sai.
- **Uma janela nativa reaberta sabe para onde cada sessão foi.** Uma sessão que
  tinha se movido com `cd`, ou para dentro do tmux ou do ssh, voltava mostrando o
  caminho, a branch e as alterações do checkout dela até o próximo `cd`; agora a
  janela lê onde cada uma esteve por último ao se conectar.
- **A janela nativa escreve os prompts dos agentes no idioma dos seus prompts.**
  Os handoffs de pull request e de issue e um lote de comentários de revisão
  eram sempre digitados em inglês.
- **Trocar o tema durante uma reconexão não derruba mais a janela nativa.**
  Trocar o tema, ou o sistema alternar entre claro e escuro, enquanto a janela se
  reconectava a um lich reiniciado podia pintar terminais que a reconexão tinha
  acabado de liberar e derrubar a janela.
- **Texto em chinês, japonês e coreano não congela mais a janela nativa no
  Linux.** Uma sessão, uma Revisão ou um pull request cheio de texto CJK podia
  travar a janela por segundos, o bastante para o "não está respondendo", e um
  zoom trazia o congelamento de volta.
- **A janela nativa usa menos memória.** O terminal de cada sessão e a Revisão
  carregavam a própria cópia das fontes de reserva, dezenas de megabytes cada
  uma para uma fonte CJK ou de emoji: seis sessões abertas agora ocupam cerca da
  metade da memória de antes. Ela também inicia mais rápido em uma máquina com
  fontes CJK instaladas.
- **Reabrir uma sessão cuja conversa já está aberta foca esse card.** Uma sessão
  fechada vinda do Histórico, um fechamento desfeito ou uma worktree existente
  podiam voltar como um segundo card em uma conversa que outro card ainda
  rodava, e os dois escreviam no mesmo transcript. O Histórico lista essa
  conversa uma só vez, pelo fechamento mais recente, e Esquecer remove todas as
  cópias dela.

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

