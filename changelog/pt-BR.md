## [0.87.0] - 2026-11-08

> [!CAUTION]
> **Todos: esta release é uma atualização obrigatória.** Antes dela, uma página em qualquer navegador da sua máquina podia ler o checkout de uma sessão pelo servidor de arquivos locais do navegador dela, o token da sessão aparecia na linha de comando de um processo, e uma sessão rodando no sandbox podia alcançar o token do próprio lich e agir na sua máquina por meio dele, então atualize agora.

### Added

- Uma release pode exigir a sua atualização. Quando uma release corrige um problema de segurança, o aviso de atualização não tem Depois e volta se você o fechar, e o lich não abre nenhuma nova sessão de agente até a atualização ser instalada: a janela, `lich open` e as ferramentas de um agente recusam, cada um dizendo por quê e como atualizar. As sessões que já estão rodando não são tocadas, e um shell ainda abre para você rodar a atualização nele. Ela conta todas as releases que você pulou, não só a última, e precisa que o GitHub esteja acessível. No diálogo Novidades uma release assim traz um destaque vermelho.
- Uma sessão pode esperar por algo e depois agir sobre isso. **Agendar prompt…** no menu dela recebe um prompt e um horário (`45m`, `2h30`, `14:30` ou uma data completa, com o momento em que ele cai mostrado enquanto você digita) ou uma condição no pull request da branch dela: as verificações falham, as verificações passam ou chega uma revisão. Nada é gasto enquanto nada muda, e o prompt é digitado na sessão do jeito que uma mensagem retransmitida é, em todos os provedores. Uma vigia de verificações falhando continua ligada a cada novo commit até o pull request fechar e para depois de 3 execuções, avisando que parou. Um relógio ou um radar na linha da sessão diz que ela guarda uma, e o tooltip diz o quê. Um agente pode armar a mesma coisa na própria sessão com `lich schedule` ou com a ferramenta MCP `schedule_prompt`, e o que você definir ali fica até você mudar.
- Um agente pode rodar JavaScript no navegador da sessão dele e ler a resposta (`browser_eval`, `lich browser eval`): o estado do seu app, um valor que a página não imprime, o resultado de um `fetch`. A resposta é cortada em 2000 caracteres, então um estado grande da página custa ao contexto do agente só o que ele pede. O script pode ler o que um campo guarda, senhas incluídas, o que a leitura da página nunca entrega a ele.
- Um agente pode ler parte de uma página em vez de toda ela: `browser_info` aceita `within`, um seletor CSS (`lich browser info --within <css>`), e responde com o texto e os botões e links do primeiro elemento que casa, então a coluna principal de um site de documentação ou um diálogo aberto custa uma fração da página e da navegação. Um seletor que não casa com nada, ou que não é válido, responde com um erro e deixa funcionando os índices da última leitura.
- A captura de tela do navegador de uma sessão pode trazer desenhado nela o índice de cada botão e link (`annotate` em `browser_screenshot`, `lich browser screenshot --annotate`), então um agente clica um ícone sem rótulo pelo índice em vez de descobrir onde ele está. O seu painel mostra os números do momento da captura.
- Qualquer programa pode dizer ao card da sua sessão o que está fazendo. Um script de build, um deploy ou um agente sem hooks do lich escreve a sequência do Program Status Protocol (OSC 7501) e o card o mostra trabalhando com a mensagem dele, aguardando você com a pergunta dele, ou concluído. Os hooks da própria sessão ainda vencem quando ela os tem. Programas que perguntam se o terminal lê o protocolo recebem a resposta. No Windows isto não foi verificado.
- Quando um limite de uso interrompe um turno do Claude Code ou do Codex e outro dos seus perfis logados do mesmo agente ainda tem cota, um aviso oferece continuar lá. Só clicar nele move a sessão, do jeito que Continuar em faz, e envia a continuação na hora; senão a sessão espera pela renovação como antes. Um perfil cujo plano o lich não consegue ler nunca é oferecido, e um clique depois que a sessão já seguiu adiante é recusado e diz por quê.
- Um agente que entrega o trabalho dele pode sugerir outro dos seus perfis para o sucessor (`profile` em `handoff_session`, `lich handoff --profile`). O sucessor ainda começa no perfil de quem chamou, e um aviso oferece o sugerido quando o primeiro turno dele termina: só clicar em Continuar em o move. Uma sugestão que não pode ser aplicada é descartada, o agente é avisado por quê, e a entrega segue.
- Um agente pode entregar o trabalho dele a uma sessão nova de si mesmo num ponto limpo, em vez de seguir num contexto que cresceu demais. A ferramenta `handoff_session` (e `lich handoff`) recebe uma nota curta, o que está feito, a branch e os commits, os próximos passos e as armadilhas, abre uma nova sessão no mesmo checkout com o mesmo provedor, modelo e perfil, e digita a nota no prompt dela. A sessão antiga continua aberta para você fechar.
- Uma sessão do Claude Code cuja conversa é compactada enquanto tarefas de outras sessões ainda esperam a resposta dela é lembrada delas quando a compactação termina: cada ticket, o que ele pediu e o comando que o responde. Antes, o resumo podia perder o ticket, e com duas tarefas abertas uma resposta que não citava nenhuma era recusada, então as sessões que perguntaram esperavam por nada.
- Duas sessões flagradas trocando mensagens entre si, de um lado para o outro, sem trabalho no meio, são paradas. A segunda mensagem dessas em sequência chega ao agente dela com uma nota de que a troca parece um loop, e a quarta é recusada, então o par para de gastar tokens à toa. Uma mensagem que vem depois de trabalho de verdade, uma terceira sessão entrando na conversa, ou você digitando em qualquer uma das sessões zera a contagem. O mesmo vale para um loop por três sessões ou mais: uma quinta mensagem repassada em sequência, cada uma vinda de uma sessão que só repassou a que recebeu, é recusada. Sessões do Crush não reportam ferramentas, então nunca são paradas.

### Changed

- Um clique duplo nos widgets próprios da janela (seleção de texto, o editor, a barra de abas) agora é registrado com até 400 ms de intervalo em vez de 200 ms, como o terminal e o painel do navegador já faziam.
- No Windows, a janela apresenta os quadros pelo modelo flip com no máximo um quadro na fila, e um redimensionamento mostra o novo tamanho na hora em vez de um quadro mais antigo esticado até ele.
- A resposta de página do navegador de uma sessão, o que `browser_eval` devolve, e a lista de abas e o identificador de página com que as ferramentas do navegador respondem agora colocam o que a página escreveu (título, texto, rótulos de elementos, erros do console, mensagens de diálogo) entre marcadores `<page …>` e `</page …>` com um código aleatório que muda a cada resposta, e `browser_info` avisa ao agente que o texto entre eles é da página e nunca são instruções. Uma página que escreve "SYSTEM: ignore previous instructions" não parece mais o lich falando, e não consegue forjar o marcador de fechamento. Isso acrescenta 31 caracteres a uma resposta.
- A captura de tela do navegador de uma sessão tem no máximo 1280 pixels no lado mais longo, então uma de um painel de 2000 pixels de largura tem 60% menos pixels, que é o que um agente paga em tokens quando olha para ela, e um clique num ponto lido nela ainda cai onde foi lido. As descrições das ferramentas do navegador, que um agente lê a cada requisição, ficaram um quinto mais curtas.
- Um clique ou tecla que deixa os botões e links de uma página como o agente os leu da última vez responde com uma linha dizendo isso em vez de a lista inteira de novo, o que cerca de três em cada dez respostas de página repetiam.
- A resposta de página do navegador de uma sessão diz para onde um link vai só quando o link não tem rótulo próprio: o agente clica um link com rótulo pelo índice, e os endereços eram um sexto do que ele lia.
- Um botão ou link no navegador de uma sessão mantém o índice dele enquanto está na página, então o índice de um agente ainda o clica depois que a página acrescenta algo acima dele ou o rola para fora da vista, e um índice de uma página que o agente deixou falha em vez de clicar no que tomou o número dele. Antes, cada leitura numerava a página de novo a partir de 0.
- A resposta de página do navegador de uma sessão tira do texto da página cada linha que é só o rótulo de um botão ou link que ela lista logo abaixo, e corta o texto em 1000 caracteres em vez de 1200: um décimo menor, com tanto do texto próprio da página quanto antes.
- Um clique, digitação e tecla do navegador de uma sessão num elemento por índice ou seletor falham antes do envio, dizendo o que está no caminho, quando outro elemento cobre o ponto de clique do alvo (um banner de cookies, o fundo de um modal: `covered by <div#consent-banner>`) ou o alvo está desativado, em vez de clicar no que está por cima ou não fazer nada e responder com a página como antes. Um clique num x e y escolhidos pelo agente não muda.

### Fixed

- O diálogo Nova pasta abre com o cursor no campo de nome, então você pode digitar o nome na hora em vez de clicar no campo primeiro.
- O prompt que o lich deixa guardado para continuar um turno que um limite de uso interrompeu volta a aparecer na sessão, e pode ser cancelado pelo menu dela. O mesmo vale para o aviso de que excluir uma sessão também excluiu um prompt agendado nela. Os dois sumiram com a janela web.
- `GIO_DAMAGE=on` redesenha num framebuffer que o lich mantém em vez de confiar na idade do buffer do driver, então um driver que a informa errado não deixa mais a janela com pixels velhos.
- No X11, um clique da roda rola uma vez em vez de duas, e o hover é limpo quando o ponteiro sai da janela, no Windows também.
- As sessões do Codex e do Kiro CLI agora são avisadas de que rodam dentro do lich e de quando abrir outra sessão para o trabalho, no idioma do seu prompt: as duas CLIs descartam o que o servidor MCP do lich diz ao modelo, então elas tinham as ferramentas e nenhuma palavra sobre usá-las. O Codex recebe isso em conversas novas, a menos que o seu `config.toml` defina `developer_instructions`, que o lich deixa em paz; o Kiro recebe quando o lich atualizar o plugin dele ali.
- O clique ou tecla de um agente logo depois de ele clicar num link de download num navegador de sessão que tinha acabado de iniciar não é mais apagado pelo recarregamento de página que o Chrome faz quando começa esse download: o lich segura a segunda ação até o recarregamento terminar, ou até o navegador ter 6 segundos.
- Uma worktree criada sem nome não falha mais com "which already exists" quando o nome que o lich escolheu para ela cai na pasta de uma worktree cuja branch foi renomeada. O lich agora pula qualquer nome cuja pasta esteja ocupada.
- Uma worktree que o git cria mas não consegue ler como um checkout é removida junto com o erro. Antes, a pasta dela ficava para trás, e criar uma worktree com esse nome de novo falhava com "which already exists".
- A navegação de um agente logo depois de uma página baixar um arquivo não falha mais com `net::ERR_ABORTED` no Chromium 157 e posteriores, que recarrega a página quase 4 segundos depois de o download começar: o lich carrega a página de novo.
- Uma tarefa enviada a uma sessão do Claude Code ou do Cursor CLI numa pasta em que ela já confia chega ao prompt dela mesmo quando a sessão subiu sem o lich-plugin, o que acontece quando o plugin é atualizado enquanto a sessão inicia. Antes, o lich segurava a tarefa esperando um relatório que a sessão nunca enviaria, e depois de 5 minutos respondia que a tarefa nunca chegou a um prompt enquanto o agente ficava parado nele.
- Uma sessão que imprimiu mais de 2 MB mantém os modos do terminal dela quando a janela reconstrói o terminal, ao mostrá-la pela primeira vez ou depois de uma reconexão. Antes, o mouse podia parar de funcionar ou voltar a uma codificação antiga, um paste podia ser enviado na primeira quebra de linha, as teclas chegavam sem o protocolo kitty, um app em tela cheia desenhava sobre a tela do próprio shell, e as cores de paleta, de primeiro plano, de fundo e de cursor que ela definira voltavam ao padrão. Vale para todos os provedores.
- Emoji feitos de vários caracteres aparecem como um só no terminal, tão largos quanto outros terminais os desenham: uma família é um emoji em vez de três, uma bandeira é a bandeira em vez de duas letras, e o resto da linha não se desloca mais para a direita. Uma letra seguida de um acento combinante mostra o acento. Copiar o texto ou abrir um link mantém todos os caracteres.

### Security

- Uma sessão rodando no sandbox não guarda mais o token do próprio lich. Ela recebe um emitido só para ela, que o lich aceita apenas para as chamadas de que um agente confinado precisa: enviar mensagens às sessões ao lado, abrir e conduzir sessões, controlar o próprio navegador, ler os serviços do projeto e o custo dele. Iniciar ou editar um serviço (que roda um comando na sua máquina, fora do sandbox), mudar uma configuração como o próprio nível do sandbox, digitar no terminal de outra sessão e toda outra chamada continuam recusados, com uma mensagem que diz por quê. Uma sessão aberta de dentro do sandbox também fica confinada, seja qual for a configuração de sandbox desse checkout, e uma sessão confinada só pode enviar mensagem, inserir, controlar ou interrogar sessões que também estejam confinadas: uma mensagem é executada pelo agente que a recebe, então uma enviada a uma sessão sem confinamento rodaria na sua máquina. As respostas chegam a qualquer um, que é como um worker confinado reporta de volta. Uma sessão confinada também não pode forçar o fechamento de uma worktree.
- O sandbox esconde o diretório do próprio lich e as worktrees das outras sessões. `~/.config/lich` (o token do lich, o banco de dados dele, os perfis das suas outras contas) e `~/.local/share/lich/worktrees` eram legíveis dentro de uma sessão confinada, como parte de `~/.config` e `~/.local/share`; agora um diretório vazio fica sobre cada um, com só o perfil da própria sessão, os arquivos soltos, os scripts de hook e o checkout montados de volta.
- O servidor de arquivos locais do navegador de uma sessão, o que serve o checkout da sessão em `http://127.0.0.1:<port>/`, agora responde só a requisições endereçadas a `127.0.0.1:<port>` ou `localhost:<port>`. Uma página no seu navegador comum podia apontar um nome que ela controla para a sua máquina e ler o checkout por ele, arquivos `.env` incluídos; uma requisição assim agora é recusada.
- O comando que o lich cola para concluir uma autoatualização envia o token da sessão ao lich num cabeçalho de requisição passado pela entrada padrão do curl, então ele não aparece mais na linha de comando de um processo, que todo usuário da máquina pode ler. O listener de loopback aceita `Authorization: Bearer <token>` ao lado de `?token=`. O mesmo vale para a reinicialização que o `install.sh` faz.
- O próprio diretório de configuração do lich (`~/.config/lich` no Linux) passa a ser legível só pelo seu usuário, a cada inicialização, então uma instalação existente também é reforçada. Prompts e o texto das sessões ficam no banco de dados que está ali. O diretório acima dele fica como estava.

## [0.86.0] - 2026-11-07

### Added

- A bandeja avisa quais sessões precisam de você. O menu dela lista as sessões que aguardam você, com o que elas estão perguntando, e as que têm um turno concluído que você ainda não leu, depois as que estão trabalhando, com a ferramenta que rodam; clicar em uma abre a janela no card dela. Passar o ponteiro sobre o ícone lista todas, sem cortes, onde a sua bandeja mostra a dica do ícone, e no macOS a dica de cada entrada traz a linha inteira. O ícone ganha um ponto quando algo está acontecendo: âmbar para uma sessão aguardando você, verde para um turno não lido, cinza enquanto uma trabalha. Os serviços foram para um submenu.
- A bandeja de notas na página do navegador de uma sessão tem uma caixa para uma nota sobre todas elas, como um lote de comentários de revisão. O que você escreve ali fecha a mensagem que o agente recebe. Enter envia as notas, Shift+Enter começa uma nova linha, e Limpar esvazia a caixa junto com o resto.

### Changed

- A janela gasta cerca de um décimo a menos de CPU em cada quadro que desenha: ela não calcula mais qual parte do quadro mudou quando vai desenhar o quadro inteiro de qualquer jeito.
- Mover o ponteiro sobre a janela não a redesenha mais a cada movimento: ela só desenha quando algo sob o ponteiro muda, e um agente que acompanha o mouse (o Claude Code acompanha) não faz mais a saída dele ser desenhada na hora conforme o ponteiro se move. Menos energia gasta enquanto sua mão está no mouse.
- Um link em um terminal fica sublinhado, e o ponteiro vira uma mão, só enquanto Ctrl (Cmd no macOS) está pressionado, a tecla com a qual um clique o abre, como no Ghostty. Antes, todo link sob o ponteiro ficava sublinhado.
- A nota abaixo de um lote de comentários de revisão, em Código, Revisão e Pull requests, agora é uma caixa que cresce conforme você escreve: Shift+Enter começa uma nova linha, e Enter continua adicionando a nota ao lote.

### Fixed

- Digitar ou colar em um terminal que você rolou para cima o traz de volta para o final, onde o que você digita aparece, como a maioria dos terminais faz. Antes, o texto chegava à sessão enquanto a visão ficava no histórico de rolagem, e você precisava rolar para baixo à mão para vê-lo.
- O texto do terminal tem o mesmo peso do resto do texto da janela. Antes, as bordas suaves de cada letra eram misturadas de outro jeito, então o texto claro em um tema escuro parecia mais fino do que deveria e o texto escuro em um tema claro parecia mais pesado.
- Uma variável que o rc do seu shell exporta com quebra de linha no valor (um certificado, uma chave) chega inteira às suas sessões. Antes, só a primeira linha chegava, e uma linha seguinte escrita como `NAME=value` aparecia como uma variável à parte.
- Um arquivo marcado como Visto na aba Arquivos da tela de Pull requests abre recolhido quando o pull request é aberto de novo, até que um novo commit o altere. Antes, ele voltava aberto.
- Abrir o lich logo depois de fechá-lo, ou a reabertura após uma atualização, não falha mais com "lich could not listen on 127.0.0.1:47821" enquanto o lich antigo ainda está parando os serviços de um projeto. Ao sair, o lich agora libera a porta antes de esperar por eles.

## [0.85.0] - 2026-11-04

### Added

- O menu de um checkout na barra lateral, aberto com o botão direito ou pelo … da linha, tem Fechar todas as sessões: todas as sessões daquele checkout fecham de uma vez, cada uma com seu próprio Desfazer, depois de uma confirmação quando alguma delas está no meio de um turno, e a última oferece remover a worktree.

### Fixed

- No Linux, a janela não vira mais pixels velhos espalhados em drivers gráficos que informam errado o que seus buffers guardam, como faz o NVIDIA 595 no COSMIC: todo quadro volta a ser desenhado inteiro, como antes da 0.84.0. Desenhar só o que mudou, com a carga menor na placa de vídeo, agora é opcional com `GIO_DAMAGE=on` no ambiente.
- Um lich compilado de uma release com um rótulo próprio na versão, como `0.83.0+local.3`, recebe a oferta da próxima release em Buscar atualizações e na verificação ao abrir. Antes, qualquer build com um `+` na versão era tomado por um build de desenvolvimento e ouvia que estava na versão mais recente.

## [0.84.0] - 2026-11-03

### Added

- A aba Conversa de um pull request na tela de Pull requests mostra a foto do GitHub de cada autor ao lado das revisões e dos comentários, numa coluna à esquerda, como o GitHub faz. Os comentários dentro de uma thread de revisão, na aba Conversa e na aba Arquivos, são separados por uma linha. Até a foto carregar, ou para uma conta que não tem uma, a inicial do autor fica no lugar dela.
- As listas de tarefas nas descrições e nos comentários de pull request mostram uma caixa de seleção, marcada ou não, no lugar de um `[x]` ou `[ ]` literal.
- Os títulos de nível mais alto nas descrições e nos comentários de pull request são sublinhados por toda a largura do texto, como no GitHub.
- A skill de tema do plugin do lich, que escreve, porta ou corrige um tema de cores do lich, agora chega ao opencode, ao oh-my-pi, ao Crush e ao Kiro CLI como `lich-theme`; antes só estava no Claude Code, no Codex, no Antigravity e no Cursor CLI. No opencode ela é o comando `/lich-theme`. Chega na próxima instalação ou atualização do plugin em Configurações › Provedores.
- O opencode 2 roda no lich. O lich o distingue do opencode 1 pela versão e o abre do jeito que o 2 espera: cada sessão num servidor próprio, de modo que o perfil, o modelo e as ferramentas do lich chegam a ele, com forks, Expandir pastes longos, logins de perfil e o medidor do OpenCode Go funcionando como no opencode 1. As conversas iniciadas no opencode 2, e as do opencode 1 que ele continuou, voltam a mostrar custo, últimas palavras e resultados de busca, e aparecem no Histórico. O spinner, o anel, o sino e o título do card precisam da próxima versão do plugin do lich, instalada em Configurações › Provedores, que agora vai para onde o opencode 2 lê plugins e toma o lugar da cópia que um lich mais antigo instalou. Uma sessão aberta pelo lich não fica no servidor compartilhado em segundo plano do opencode 2, então `opencode pair` e o app desktop não a enxergam.
- O título Descrição da Visão geral na tela de Pull requests tem um menu ⋯, como os comentários do GitHub: Copiar link copia o endereço do pull request, Copiar Markdown copia a descrição como foi escrita, e Editar abre a descrição para edição, o que antes o botão Editar ao lado do título fazia.
- Um agente pode gerenciar os serviços de um projeto como o rodapé de Serviços faz: listar, criar, editar, excluir, iniciar, parar, forçar a parada e reiniciar um serviço, iniciar ou parar todos, e ler o que um serviço imprimiu, rodando ou parado, com as ferramentas MCP `*_service` ou com `lich service` num terminal. Um serviço é nomeado pelo nome dele no projeto, o do próprio agente, a menos que ele cite outro.

### Changed

- No seu próprio pull request, a tela de Pull requests não oferece mais Aprovar nem Pedir alterações, que o GitHub recusa ao autor, e o menu do diff não oferece mais um comentário no pull request. O menu do diff também deixa de fora Comentar no pull request quando nenhuma linha está selecionada, em vez de mostrá-lo desativado como antes.
- As descrições, os comentários e as prévias de Markdown de pull request ficaram mais fáceis de ler, com texto mais claro, títulos e links mais nítidos, código em linha destacado, mais espaço entre linhas e blocos, e itens de lista quebrados alinhados com o texto deles.
- A tela de Pull requests abre num índice dos pull requests do projeto ocupando a tela inteira, e um pull request abre no lugar dele em vez de ao lado da lista. Um botão de voltar sobre o título retorna ao índice como você o deixou; ao lado dele, o número abre um menu dos outros pull requests, e anterior e próximo percorrem o filtro do índice. Na aba Arquivos só a árvore de arquivos fica ao lado do diff, que ganha a largura que a lista ocupava. A Visão geral põe a descrição ao lado de uma coluna com os revisores, as verificações, a possibilidade de merge e o checkout em que a branch está; a faixa de abas termina com o tamanho do pull request. Abrir em sessão e Merge ficam no cabeçalho, com Resolver conflitos ou Corrigir erros de CI ao lado quando o pull request precisa de um, e Aprovar, Atualizar e Abrir no GitHub passam para o menu ⋯. A tela se alinha ao espaçamento da barra lateral, seus botões têm o tamanho dos do resto da janela, e o código de que uma thread de revisão trata fica num bloco próprio, separado dos comentários. No índice, j e k ou as setas movem, Enter abre e / vai para o filtro; num pull request, Esc volta e Alt com uma seta passa ao pull request anterior ou ao próximo.
- O tema Dark que acompanha o lich dá à interface e ao terminal o mesmo fundo carvão neutro, eliminando o tom azul-escuro do terminal, e usa uma paleta de terminal e de sintaxe mais viva, de verde, coral, âmbar, azul, violeta e ciano. O tema Light usa versões mais escuras dessas cores, ajustadas para o contraste no fundo claro do terminal.
- O navegador de uma sessão não é mais fechado sem perguntar. Quando nenhum painel o mostra e ninguém o usa há cinco minutos, um toast pergunta se deve fechá-lo: Fechar o fecha, e Manter aberto, ou dispensar o toast com um deslize, o mantém por mais cinco minutos. Sem resposta, o toast fica na tela e o navegador fica aberto, então uma página deixada aberta durante o almoço continua lá quando você volta. Usar o navegador, ou o agente usá-lo, tira o toast da tela. Antes, o navegador era fechado após cinco minutos longe da sessão dele, e a página, as abas e o login se perdiam.
- Na tela de Pull requests, o botão que envia as threads de revisão abertas de um pull request para a sessão dele fica no cabeçalho ao lado de Abrir em sessão, em todas as abas, do tamanho dos botões ao redor. Antes era um botão pequeno no topo da aba Conversa, fora de vista quando a conversa rolava.
- A faixa de redimensionamento da barra lateral fica na própria borda direita dela, como a do painel Código e Revisão, em vez de no vão entre a barra lateral e as sessões.
- A barra lateral é uma árvore de sessões. Uma sessão é uma linha: o anel dela, o nome, o pull request e as alterações, e um canto onde um … sob o ponteiro abre o menu da sessão, com fechar incluído. O que as outras linhas do card diziam foi para o tooltip dele: a ferramenta que o turno roda ou o que ele espera, há quanto tempo, o caminho e o modelo. Cada checkout é uma linha própria, com a branch, o pull request e as alterações, e as sessões dele pendem dela. Uma sessão aberta por outra (um fork, uma sessão aberta com `open_session` ou `lich open`, um card de subagente) pende da sessão que a abriu, mesmo rodando em outra worktree, e uma mãe recolhe os filhos, mantendo a contagem e a marca de aguardando ou não lida. As linhas se movem pela sua linha: quando uma mãe se abre, uma sessão abre ou uma worktree ou uma pasta aparece, uma linha se abre já com o nome nela e a linha desce até ela e desenha o anel; recolher ou fechar tira o nome, apaga o anel e sobe a linha de volta, e a saída da última sessão de uma worktree ou de uma pasta leva a linha da worktree, ou a pasta, embora depois dela. Uma pasta fica numa bandeja, então se vê onde ela termina. Fixar foi para o menu do card, que também copia o comando `lich reply` de um pedido que a sessão deve. Um projeto sem worktree mantém uma lista plana. A barra lateral recolhida mostra a mesma árvore numa coluna: o nó de cada checkout, a linha passando por suas sessões, o anel menor de um filho na linha da mãe, e cada pasta na sua bandeja, com as linhas entrando e saindo pela mesma linha. Arrastar uma sessão solta a linha dela, mostra um espaço tracejado onde ela vai se juntar à árvore e desenha a linha de volta até ela quando pousa; a pasta ou worktree em que ela seria arquivada acende inteira. Uma mãe arrastada leva os filhos com ela, e uma sessão só pousa onde a árvore a mantém, nunca entre uma mãe e seu filho. Uma sessão arquivada numa pasta que tem cor assume essa cor, então a pasta a mantém, seja arrastando a sessão para lá, escolhendo a pasta no menu dela, ou um agente arquivando com `file_session` ou `lich file`; uma sessão aberta numa pasta, o worker de um subagente ou a sessão que o pediu também a usa. Soltar uma sessão numa pasta mais abaixo na barra lateral a arquiva de novo, onde antes ela voltava ao seu lugar quando uma worktree acima também a recebia. A barra lateral recolhida também arrasta sessões, e o botão de busca dela abre o mesmo filtro da barra aberta, que continua filtrando o trilho depois que o painel fecha.
- Na Visão geral da tela de Pull requests, o título Descrição e o ⋯ dele ficam no lugar no topo da coluna enquanto a descrição rola por baixo, sobre um separador, e o ⋯ fica na borda direita da coluna.
- Os placeholders de carregamento da tela de Pull requests combinam com o que carrega no lugar deles: o índice mostra linhas da altura das reais, com a coluna de verificações onde ela cai, preenchendo a tela, e um pull request mostra a linha do título com seus botões, a faixa de abas real e o layout da aba aberta (a descrição ao lado dos fatos, os commits, as verificações, os avatares e comentários da conversa, ou a barra de ferramentas, a árvore e os cards dos arquivos), em vez de algumas barras genéricas. A conversa e o diff mostram os mesmos placeholders enquanto carregam, no lugar de uma linha de texto.

### Fixed

- Uma revisão, resposta ou outra ação de pull request que o GitHub recusa diz o motivo, como uma revisão pendente deixada aberta no github.com ou um comentário numa linha fora do diff. Antes, toda recusa dizia "gh could not complete the request", porque o motivo enviado pelo GitHub era descartado.
- Um comentário de revisão pendente sob a sua linha na aba Arquivos da tela de Pull requests tem Editar e Descartar, como na aba Conversa. Antes, só podia ser alterado ou descartado pela aba Conversa.
- Na aba Arquivos da tela de Pull requests, expandir as linhas escondidas entre as alterações, visualizar um arquivo Markdown e mostrar uma imagem funcionam num pull request cuja branch não está em checkout em lugar nenhum. Eles falhavam com "Couldn't read" o arquivo, porque o lich o procurava numa pasta com o nome do endereço do pull request.
- A aba de um pull request na tela de Pull requests abre no topo. Antes, abria onde a aba ou o pull request anterior tinha sido rolado, então a aba Conversa podia abrir no fim.
- O código citado numa thread de revisão na tela de Pull requests mostra a indentação em vez de uma caixa para cada tab.
- Um campo de arquivo no navegador da sessão, como o envio de uma foto de perfil, abre o seletor de arquivos do sistema ao clicar, e aceita vários arquivos quando o campo permite. Antes, o clique não fazia nada.
- Sair de uma página do navegador da sessão que pergunta antes de você ir não recarrega mais a página no lugar numa máquina ocupada; o navegador vai para onde estava indo.
- Um agente que clica num ponto do navegador da sessão não erra mais o ponto depois que você move o lich para outro monitor, redimensiona o painel do navegador ou muda o zoom dele. As capturas de tela dele agora estão nos mesmos pixels que os cliques, que um monitor HiDPI ou um zoom de página tinham deixado até 1,5 vez maiores, e um clique num ponto lido de uma captura tirada antes de o painel mudar de tamanho é recusado, dizendo ao agente para tirar uma nova.
- O lich não mantém mais a GPU ocupada enquanto uma sessão trabalha. O spinner de uma aba de projeto redesenhava a janela inteira na taxa de atualização do monitor, e com sessões ocupadas em mais de um projeto o lich segurava mais da metade da GPU de um notebook, tirando-a dos jogos e de tudo o mais. Todo spinner, o anel de ocupado da barra lateral e a pulsação de um serviço em execução agora redesenham no máximo 24 vezes por segundo, e 8 vezes por segundo enquanto a janela do lich não está em foco, então o anel de uma sessão ainda é visto girando de outro monitor. O ponto de um serviço em execução pulsa a cada seis segundos em vez de a cada dois e meio: cada pulso é uma rajada de quadros, e o ritmo antigo eram oito quadros por segundo o dia inteiro para um ponto.
- Enquanto uma sessão trabalha, a janela desenha 24 quadros por segundo, a taxa do cinema, em vez de 30, e a saída do terminal da sessão é desenhada no próximo quadro do anel de ocupado em vez de pedir quadros próprios: um agente transmitindo e um anel girando custam uma taxa de quadros, não a soma das duas. O que você digita continua aparecendo no instante em que o terminal ecoa.
- O terminal não redesenha mais a linha do cursor a cada quadro enquanto o cursor fica parado; a linha é recomposta quando o cursor se move ou o texto dela muda.
- No Linux, a janela desenha só o que mudou desde o último quadro e informa essa área ao compositor, em vez de repintar tudo 24 vezes por segundo enquanto uma sessão trabalha: um quadro em que só o anel de ocupado girou desenha o anel e a linha do cursor do terminal, e o trabalho da placa de vídeo no renderizador da janela cai para um quinze avos. O Windows e o macOS continuam desenhando quadros inteiros. `GIO_DAMAGE=off` no ambiente desenha quadros inteiros também no Linux, para um driver gráfico que informa errado o que seus buffers guardam.
- A janela do lich usa metade do processador que usava enquanto uma sessão trabalha. Todo ícone, anel e arco na tela era reconstruído a partir do contorno a cada quadro; agora cada um é construído uma vez e mantido na placa de vídeo.

## [0.83.0] - 2026-10-29

### Added

- O terminal desenha imagens enviadas com o protocolo gráfico do Kitty e informa aos programas o tamanho de suas células em pixels (`CSI 14 t`, `16 t`, `18 t`). Uma TUI que mostra ícones ou prévias assim, como o gerenciador de arquivos `tfm`, agora os mostra numa sessão do lich como faz no Kitty ou no Ghostty. Antes, esses programas recorriam a pequenos glifos de texto ou não desenhavam nada. Imagens enviadas como Sixel ou pelo tmux ainda não são desenhadas.
- O navegador da sessão tem abas. Elas ficam no cabeçalho do navegador, onde um + abre uma nova; clique numa aba para trocar para ela, ou no × dela para fechá-la. O que uma página abre numa nova aba ou janela se junta a elas, na frente, e fechá-la traz de volta a página que a abriu, como estava. O agente lista e troca de aba com as novas ferramentas MCP `browser_tabs` e `browser_tab` (ou `lich browser tabs` e `lich browser tab`), e o cursor dele vai para a aba em que age.
- `lich insert --focused` cola na sessão cujo card está na tela da janela do lich: o card ativo da aba de projeto da frente. Uma ação do gerenciador de arquivos ou um editor pode enviar um caminho ao agente que você está olhando sem adivinhá-lo pela pasta do arquivo. `lich sessions --json`, `lich whoami --json` e a ferramenta MCP `list_sessions` marcam essa sessão com `"focused": true`. Com a janela fechada, nenhuma sessão é marcada e `--focused` avisa isso.

### Changed

- O que uma página do navegador da sessão abre numa nova aba ou janela não substitui mais a página que a abriu. Antes, o endereço da nova aba carregava por cima dela, então um login ou qualquer outra coisa que a página guardava no script se perdia, e um fluxo que abre uma aba e espera ela terminar quebrava.

### Fixed

- Fechar uma sessão cujo navegador estava aberto ao lado dela, com o terminal em foco, não fecha mais a janela do lich.

## [0.82.0] - 2026-10-29

### Added

- `lich sessions --json`, `lich whoami --json` e a ferramenta MCP `list_sessions` trazem a linha que o card de uma sessão mostra sob o rótulo dela: `tool` e `detail` enquanto ela está ocupada, a ferramenta que roda e sobre o que age, e `reason` enquanto ela espera por você. Um script ou um widget de desktop pode mostrar o que cada agente está fazendo sem abrir a janela.
- O navegador de uma sessão abre um arquivo local do checkout da sessão. Digite o caminho no campo de endereço, ou o agente passa o caminho ou uma URL `file:` para `browser_open`, e o lich serve o checkout por http em 127.0.0.1, numa porta própria por worktree (o `LICH_WORKTREE_PORT` do checkout mais 1000), então scripts de módulo, `fetch` e recursos relativos funcionam e cada worktree mantém o seu próprio armazenamento. Um caminho fora do checkout é recusado. Antes, um arquivo local não podia ser aberto, e visualizar um significava subir um servidor estático à mão.
- A barra de ferramentas do navegador de uma sessão tem um botão de zoom. Ele abre um menu pequeno que diminui e aumenta o zoom da página em 10% e volta a 100%, cada controle com o nome do seu atalho de teclado. Antes, o zoom só podia ser mudado pelo teclado.

### Fixed

- Continuar em mantém o navegador da sessão. Mover uma sessão para outro perfil fechava o navegador dela, e a página e os cookies sumiam quando a sessão voltava.
- O navegador da sessão acompanha uma página ocupada. Numa página que anima ou roda muito JavaScript, a aba do navegador ficava segundos atrás da página e o resto da janela esperava junto; agora ele pula para o quadro mais recente.
- O navegador da sessão fica nítido numa tela HiDPI. A aba mostrava a página em 1x, borrada, em qualquer monitor; agora a desenha em até 1,5x.
- As notas do navegador da sessão, e o Corrigir erros de CI e o Resolver conflitos da tela de Pull requests, chegam a uma sessão do Claude Code que você parou com Esc. O Claude Code recusava a mensagem até alguém digitar na sessão, e a bandeja dizia "its mod refused it"; agora o lich digita a mensagem no prompt.

## [0.81.0] - 2026-10-28

> [!IMPORTANT]
> **Quem tem um agente trabalhando numa página web: aponte para a página e diga o que mudar.** A mira na barra de
> ferramentas do navegador da sessão deixa notas nos elementos que você escolhe e as envia ao agente em uma só mensagem.

### Added

- A mira na barra de ferramentas do navegador de uma sessão deixa notas na página para o agente. O elemento sob o ponteiro é contornado e nomeado, com o contorno deslizando de um elemento para o outro conforme o ponteiro se move; um clique abre uma nota ao lado dele, e cada nota fixa o seu elemento com um número, acompanhando-o enquanto a página rola. As notas ficam numa bandeja flutuando sobre a parte de baixo da página, que nunca é redimensionada sob você. Passar o ponteiro sobre uma nota ali acende o elemento dela, e um clique nela ou no seu número rola o elemento para a vista e edita a nota. Um elemento cortado por uma lista com rolagem, uma barra lateral ou um diálogo é contornado só onde aparece, e um que fica escondido não mantém o número sobre o que está ao lado. Enviar (ou Ctrl+Enter) digita as notas na sessão como uma só mensagem: qual elemento, um seletor que o `browser_click` do agente aceita, o componente React que o desenhou numa build de desenvolvimento, e a sua nota. Ctrl+Shift+E (Cmd+Shift+E no macOS) liga e desliga a mira, e Esc sai dela. Um envio que a sessão recusa mantém as notas e diz o motivo na bandeja.
- O navegador de uma sessão baixa arquivos. Um download iniciado pela página, por um link, um botão, uma nova aba ou um script, cai na sua pasta Downloads com o nome que a página dá, com ` (1)` acrescentado em vez de sobrescrever um arquivo que já esteja lá. O agente também pode abrir a URL de um arquivo diretamente. O clique, a tecla pressionada ou a abertura do agente respondem com o caminho e o tamanho do arquivo, e a janela mostra um toast com o nome do arquivo, com um botão que abre a pasta dele. Antes, um botão de download no navegador da sessão não fazia nada.
- O agente pode esperar a página do navegador mudar em vez de lê-la de novo num laço. O `browser_info` aceita `wait_for` (texto que deve aparecer) e `wait_gone` (texto que deve sumir), procurados em qualquer lugar da página, e responde assim que a página o mostra, ou depois de 20 segundos com a página como está e uma linha dizendo que a espera esgotou o tempo.
- O agente fica sabendo quando a página no navegador da sessão quebra. Cada página que ele lê, clica ou em que pressiona uma tecla responde com os erros de console desde a última olhada, uma vez cada: chamadas a `console.error`, exceções não capturadas e recursos que falharam ao carregar, cortados em uma linha curta, os cinco primeiros e quantos mais. Avisos e outras mensagens ficam de fora.
- Um agente pode fechar um projeto, com `close_project` ou `lich close-project`: a aba sai da janela e as sessões dela param, guardadas para reabrir como quando você fecha a aba. O projeto em que o agente roda e o projeto inicial são recusados.
- O agente e você respondem aos diálogos de JavaScript da página no navegador de uma sessão. Um alert, um confirm, um prompt ou um "Sair desta página?" que a página abre aparece como um cartão sobre o topo da página, na aba do navegador da sessão, nomeando o site que o abriu, com os botões dele e, para um prompt, um campo. Você o responde ali, ou o agente com `browser_dialog`, e o cursor dele pressiona o botão do cartão. O clique do agente que abriu um diálogo responde na hora com o diálogo dentro, e o agente fica sabendo de um que você respondeu. Antes, um botão que mostrava um alert congelava a página: o clique do agente e toda leitura depois dele esperavam trinta segundos e falhavam.
- Configurações › Geral tem um interruptor Copiar ao selecionar para o terminal. Ligado por padrão, copia a seleção assim que você a faz; desligado, copie com Ctrl+Shift+C (Cmd+C no macOS).
- O navegador de uma sessão dá zoom na página por conta própria, separado do zoom da interface: Ctrl+- e Ctrl+= (Cmd no macOS) diminuem e aumentam em 10%, de 50% a 200%, e Ctrl+0 volta a 100%. Uma página com zoom menor ocupa mais largura, como num navegador desktop, o que torna uma tela pequena utilizável sem encolher o resto da janela. O zoom vale para o navegador de toda sessão, aparece na barra de ferramentas enquanto não é 100% e é guardado para o próximo início.
- Configurações › Geral › Janela › Botões escolhe se os botões de minimizar, maximizar e fechar da janela aparecem: Seguir o sistema, Sempre mostrar ou Esconder. Seguir o sistema é como o lich se comportava antes, e no Linux agora também acompanha uma mudança no layout de botões da área de trabalho enquanto o lich está aberto, em vez de só lê-lo no início. Sempre mostrar desenha os botões mesmo onde o gerenciador de janelas decora a janela ou a organiza em mosaico sem barra de título, como o Hyprland faz. Esconder deixa a janela sem eles, inclusive os semáforos no macOS.

### Changed

- A conta do GitHub de um projeto (Configurações › Controle de versão) agora chega também às sessões dele: o `gh` rodado pelo agente ou num card de shell responde como essa conta, em vez de qualquer uma que o gh tenha ativa, a partir do próximo início da sessão. Dentro dessas sessões, `gh auth switch` não muda mais a conta.
- A janela guarda menos memória com muitas sessões abertas. Um terminal fora da tela libera os pixels que desenhou, alguns megabytes cada, e os redesenha em um quadro quando volta.
- O coletor de lixo da janela agora roda em 50, e não no padrão 100 do Go, o que num teste com 15 sessões cortou cerca de um décimo da memória residente, a folga que o sistema mandaria para o swap. `GOGC` ainda tem precedência.
- As ferramentas do navegador respondem a um agente no idioma de prompt da sessão: a legenda da lista de elementos, o aviso de que há mais elementos abaixo, as marcas de preenchido e marcado, um download ainda em andamento, uma espera que esgotou o tempo, a contagem de erros de console adicionais e o que um diálogo aberto pede dele. As etiquetas de linha (`url:`, `text:`, `downloaded:`, `console error:`, `dialog open:` e as demais) continuam como estão em todos os idiomas.
- O navegador de uma sessão que nenhuma aba mostra monta a página no tamanho da janela do lich, no zoom do navegador, em vez dos 800×600 do Chrome, então o agente lê e captura a página como um navegador desktop a mostra. Fechar a aba do navegador devolve a página a esse tamanho, em vez de deixá-la no da aba.
- Resolver conflitos na tela de Pull requests nomeia os arquivos em conflito no prompt que envia à sessão, como Corrigir erros de CI nomeia as verificações que falham. Clicado antes de os arquivos serem conhecidos, envia o prompt sem eles, como antes.

### Fixed

- No Windows, a janela aparece como `lich` na barra de tarefas e no Alt-Tab, em vez de `lich (native slice)`.
- Um projeto cuja conta do GitHub vive num servidor GitHub Enterprise passa a ter essa conta na tela de Pull requests, nos selos de pull request e numa sessão em sandbox com o token dela. O token saía sob uma variável que o gh ignora para um host enterprise, então o gh respondia ali como a conta ativa dele.
- O clique, a digitação ou a tecla pressionada por índice no navegador de uma sessão chega ao elemento que a última leitura de página do agente deu a esse índice. Depois de uma rolagem e de uma segunda leitura, um elemento que a primeira leitura listava sob o mesmo índice, agora fora da vista, podia receber o clique no lugar, sem nenhum erro.
- No Windows, sessões que começam ao mesmo tempo num mesmo perfil não falham mais de vez em quando com `Access is denied` enquanto o lich grava os arquivos desse perfil.

## [0.80.0] - 2026-10-27

> [!IMPORTANT]
> **Quem tem um agente trabalhando numa página web: agora ele controla um navegador ao lado da sessão.** Abra
> Navegador no menu do card da sessão e veja o cursor do agente clicar, digitar e rolar na página, enquanto você também a usa.

### Added

- O menu de um card de sessão abre um navegador para essa sessão, acima de Abrir em, em uma aba ao lado da sessão: o terminal dela de um lado e a página do outro, com o divisor, o giro e o zoom de uma divisão. O agente da sessão controla a mesma página: abrir uma URL, lê-la, clicar, digitar, pressionar uma tecla, voltar e avançar, rolar e tirar capturas de tela, por ferramentas `browser_*` ou por `lich browser`. O cursor do próprio agente mostra onde ele age: a seta do lich, na cor do card da sessão ou na do tema, com um selo com o nome do provider, desliza até cada clique, campo e rolagem na página e até Voltar, Avançar, Recarregar e o endereço na barra de ferramentas do painel, executando em sequência até as chamadas paralelas de um agente. Você clica, digita e copia e cola com a sua própria área de transferência na página enquanto ele trabalha. Cada clique e tecla que o agente envia responde com a página que ele deixou, em forma compacta, então uma sessão longa gasta cerca da metade dos tokens que gastaria lendo a página de volta como JSON. Quando o agente abre um navegador por conta própria, um toast no canto inferior direito se oferece para mostrá-lo. Abrir em janela move a página para uma janela do Chromium própria, cookies incluídos. Fechar o card fecha o navegador. O navegador é o Chrome, Edge, Chromium, Brave, Vivaldi ou Helium já instalado.
- O rodapé mostra quanto do último turno foi lido de volta do cache de prompt, com uma barra que divide a entrada do turno em lida do cache, gravada no cache e fora do cache. Um turno depois que o cache expirou mostra 0% em âmbar, que é o turno cobrado pela tarifa cheia. Ele fica na dica da janela de contexto no Claude Code e no Codex, e na dica de custo no oh-my-pi e no opencode.

### Changed

- Uma sessão do opencode recebe o servidor MCP do lich ao iniciar, como o Claude Code e o Codex, então lista todas as ferramentas do lich, as `browser_*` incluídas, sob o servidor `lich` na lista de MCP do opencode. Os seus próprios servidores e qualquer `OPENCODE_CONFIG_CONTENT` que você exporte são mantidos.
- Um agente pode mudar o modelo ou o esforço de raciocínio da sessão em que roda por `control_session`, e fechar essa sessão quando você pedir (`self` em `close_session`, `lich close --self`). As duas coisas eram recusadas de saída.

### Fixed

- Abrir, recarregar, voltar e avançar no navegador de uma sessão retornam assim que a página fica interativa, em vez de esperar cada imagem e rastreador terminar. Essa espera segurava a próxima ação do agente pelo tempo que o evento de carga levasse, até 30 segundos, com a página já na tela. Uma captura de tela é a área visível que está sendo observada.
- O custo de uma sessão do Codex não cobra mais duas vezes os tokens gravados no cache de prompt. Eles eram precificados pela tarifa de gravação no cache e de novo como entrada fora do cache.
- A roda do mouse em um terminal rola com suavidade e tão longe quanto em outros terminais. Um clique move três linhas do histórico do shell, ou de um programa em tela cheia como `less`, suavizado ao longo de alguns quadros em vez de pular cerca de oito linhas de uma vez. Um programa que captura o mouse, como nvim, Claude Code ou Codex, recebe um passo de roda por clique e rola a própria distância, onde antes recebia cerca de oito e disparava pela conversa.

## [0.79.0] - 2026-10-24

> [!IMPORTANT]
> **Quem deixa um servidor de desenvolvimento, um container ou um túnel aberto ao lado dos agentes: agora o lich
> roda isso.** Salve uma vez em Configurações › Serviços do projeto e inicie, pare e leia cada um no fim da barra lateral.

### Added

- No macOS, o lich ganha um item na barra de menus: um lich com a janela fechada mostra ali que ainda está rodando as sessões, com Mostrar o lich, a contagem de sessões em execução e Encerrar o lich, como a bandeja faz no Linux e no Windows.
- **Serviços: comandos que um projeto mantém rodando, iniciados pelo lich.** Salve um container, um túnel ou um servidor de desenvolvimento uma vez em Configurações › Serviços de um projeto e inicie-o pela linha Serviços no fim da barra lateral, acima do perfil. Fechada, a linha diz quantos estão rodando (2/3); aberta, lista cada serviço com há quanto tempo roda ou como terminou, e Iniciar, Parar e Reiniciar ao passar o mouse. Um clique abre o console dele, onde ele aceita entrada e Ctrl+C o para. Um serviço não é um terminal: quando o comando dele sai, o serviço parou, a última saída continua legível e Iniciar o executa de novo. Marque Iniciar com o lich e ele inicia toda vez que o lich inicia com o projeto aberto. Os serviços salvos em Configurações › Serviços com Global selecionado rodam na sua pasta home e aparecem na tela inicial do lich, o lugar para um `lazydocker` que você mantém aberto seja qual for o projeto. Um serviço vive enquanto a aba do projeto existir: fechar a aba o para, e reabrir o projeto inicia os salvos para iniciar com o lich. Fechar a janela os deixa rodando, e a bandeja os lista com Mostrar saída, Reiniciar e Parar; encerrar o lich os para, e a caixa de diálogo de fechamento os conta. Com a barra lateral recolhida, o botão Serviços da barra de ícones abre a mesma lista.

- **Envie as threads de revisão de um pull request para a sessão dele.** Na tela Pull requests, uma thread de revisão aberta tem Enviar para a sessão ao lado de Responder e Resolver, na aba Conversa e no diff, e a aba Conversa pode enviar todas as threads abertas de uma vez. As threads vão como um único prompt para a sessão no checkout da branch, que o lich abre quando não há nenhuma, com o arquivo, as linhas, o link e todos os comentários de cada thread. A tela Pull requests continua aberta, e um toast com Abrir avisa quando a sessão as recebeu. O lich então responde a cada thread no GitHub com "Fixing this. It'll be in the next commit." e a resolve quando o pull request recebe um novo push, respondendo com o commit em que ela foi parar; até lá a thread aparece como Enviada, aguardando um push. Qualquer push a resolve, mesmo um que não a tenha tocado.

### Changed

- Um pull request mergeado fora do lich, no github.com, com `gh` em um terminal ou por um agente, agora é anunciado aos agentes no checkout da branch dele como um mergeado pela tela Pull requests, em até um minuto depois do merge. Antes eles continuavam achando que ele estava aberto. Ouvem os agentes que estavam rodando ali quando o lich o viu aberto e depois mergeado; um parqueado nesse momento, ou um pull request mergeado antes de o lich iniciar pela última vez, não ouve.
- Resolver conflitos e Corrigir erros de CI na tela Pull requests não levam mais você à sessão para a qual entregam o pull request. A tela Pull requests continua aberta para você passar ao próximo pull request, a sessão inicia na barra lateral, e um toast avisa quando o agente recebeu a tarefa, com Abrir para ir até ela. Uma sessão cujo início precisa antes da sua resposta, como retomar ou não a última conversa, ainda aparece. Abrir em sessão continua indo até a sessão.
- Os filtros rápidos sobre a lista de pull requests na tela Pull requests (Todos, Prontos, Rascunhos, Falhando) ficaram mais leves: texto pequeno com o selecionado sombreado, como nas abas do dock, dividindo a largura da lista. Um filtro sem nada fica apagado e a contagem de Falhando fica vermelha quando uma verificação falha. Eles não passam mais da borda da lista com uma fonte monoespaçada na interface.

### Fixed

- O selo de pull request em um card de sessão, o item de menu Pull request dele e o botão de pull request do rodapé não abrem mais o pull request errado depois que o shell da sessão passou para outro repositório. O selo mostra o pull request desse repositório, e o clique procurava o número dele entre os do próprio projeto na tela Pull requests; agora ele abre o pull request no GitHub.
- A barra lateral recolhida mostra o perfil em que as novas sessões nascem, no fim da barra de ícones, e abre o mesmo menu do rodapé da barra lateral aberta. Antes ela não mostrava perfil nenhum.
- Uma aba da barra superior não pode mais ficar presa levantada, desenhada na cor de destaque como se estivesse sendo arrastada, sem que nenhuma aba voltasse a se mover até o lich reiniciar. Uma aba focada por um clique e depois acionada com Espaço ou Enter era pega para reordenar pelo teclado, e depois que um clique em outro lugar levava o teclado ela não podia ser solta nem cancelada; agora ela volta ao lugar quando perde o foco. Uma aba de layout renomeada no meio de um arrastar também não fica mais presa assim.
- Um card de sessão não cresce e encolhe mais quando o agente começa e termina uma ferramenta, espera por você ou compacta, empurrando para cima e para baixo todos os cards abaixo dele. Essa linha agora ocupa o lugar do caminho do card enquanto aparece, e o caminho volta quando ela some.
- A janela nativa usa menos memória para as fontes. Um arquivo de fonte pedido mais de uma vez, como a interface e o terminal na mesma família ou a face negrito que também faz as vezes de seminegrito, era processado de novo a cada vez, cerca de 30MB para uma face Nerd Font; agora é processado uma vez e compartilhado enquanto está em uso. Uma interface em um idioma que não seja chinês, japonês ou coreano não carrega mais uma fonte CJK na inicialização, dezenas de megabytes: ela carrega no primeiro texto CJK na tela, desenhado nesse meio-tempo com uma fonte do sistema. Com uma Nerd Font para a interface, o terminal e o editor e sem CJK na tela, as fontes ocupam 119MB onde ocupavam 246MB.
- No macOS, abrir o lich pelo Dock, Launchpad, Spotlight ou Finder enquanto ele roda com a janela fechada abre a janela de novo. O lich iniciado no login com `--no-window` está incluído, onde o clique não fazia nada. Sair da sessão agora encerra o lich como `lich quit` faz, então a próxima inicialização não o lê como uma falha.

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

