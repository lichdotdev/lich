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
