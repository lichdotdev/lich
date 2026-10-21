## [0.76.0] - 2026-10-21

### Added

- Um arquivo Markdown aberto na aba Código tem o mesmo botão de prévia da aba Revisão, ao lado da marca de somente leitura, e mostra o arquivo renderizado, com tabelas e links, no lugar do código-fonte. Clique de novo para voltar ao código-fonte. Qualquer outro arquivo abre como código-fonte.

### Changed

- Enviar em um lote de comentários de revisão, nas abas Revisão e Código e na tela Pull requests, envia os comentários à sessão como o prompt dela, onde antes ficavam no prompt esperando um Enter. Uma sessão de shell mantém os comentários, já que ali um Enter os executaria como comando.

### Fixed

- Teclas mortas compõem no terminal de uma sessão: no macOS, ~ e depois a digita ã, e ' e depois c digita ç, onde antes o acento chegava sozinho antes da letra. O texto que um método de entrada ainda está compondo, como o de um IME japonês ou chinês, também não chega mais ao programa antes de ser confirmado.
- Depois de trocar de modelo em uma sessão do Claude Code com `/model`, o rodapé e o tooltip do card da sessão mostram o novo modelo assim que o primeiro turno dele termina. Antes mantinham o antigo até um turno posterior ou uma reconexão, porque o lich lia o transcript antes de o Claude Code ter escrito a última linha daquele turno.
- No macOS, os botões de fechar, minimizar e zoom continuam centralizados na barra de título. O macOS os devolvia ao topo da barra quando a janela abria, quando você trocava para outro app e voltava, e quando o sistema alternava entre claro e escuro, até a janela ser redimensionada.
