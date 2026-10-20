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
