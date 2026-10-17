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
