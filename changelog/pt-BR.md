## [0.72.0] - 2026-10-19

### Changed

- A barra superior da janela nativa agora é a barra de título: a barra do sistema sumiu, e minimizar, maximizar e fechar ficam ao lado de Configurações, desenhados como o Windows, o GNOME e o KDE os desenham e no lado em que o seu desktop os coloca; no macOS os semáforos ficam na barra superior. Arraste a parte vazia da barra superior para mover a janela e dê um duplo clique nela para maximizar. Gerenciadores de janela tiling mantêm as próprias decorações.

### Fixed

- `lich quit` não falha mais com "EOF" quando o lich fecha antes de responder: ele espera a porta fechar em vez de tratar a resposta perdida como recusa.
- No Wayland, a janela nativa mostra o cursor do próprio desktop, no tema e no tamanho dele, onde podia mostrar outro tema. Num monitor com escala menor que a do maior, o cursor não pula mais ao entrar na janela.
- No Linux, a janela nativa se agrupa sob o ícone do próprio lich no dock, onde aparecia como um app separado com um ícone genérico.
