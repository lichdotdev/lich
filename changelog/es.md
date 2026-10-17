## [0.70.0] - 2026-10-17

### Added

- **Novedades habla tu idioma.** El diálogo que se abre tras una actualización
  muestra las notas de la versión, y los nombres de sus grupos, en el idioma
  de la interfaz que elegiste, en ambas ventanas. Una versión cuyas notas aún
  no tienen traducción las muestra en inglés.
- **Bifurca una sesión en la ventana nativa.** Haz clic derecho en una tarjeta
  y elige Bifurcar › Aquí para continuar su conversación en una tarjeta nueva
  justo debajo, en el mismo checkout, o Bifurcar › Worktree… para llevarla a
  una worktree nueva junto con tus cambios sin confirmar. La copia lleva el
  nombre del original ("auth-refactor 2") y una marca de bifurcación. Solo
  Claude Code, Codex y opencode: los demás proveedores solo reanudan.
- **La ventana nativa avisa cuando dos tarjetas ejecutan una misma
  conversación.** Reanudar, desde la TUI de un agente, una conversación que
  otra tarjeta ya tiene abierta muestra un aviso en la esquina inferior
  derecha; Cambiar cierra la tarjeta que reanudó y trae la que la tenía.
- **La ventana nativa muestra imágenes.** Un PNG, JPEG, GIF, WebP o BMP se
  abre en la vista previa de la pestaña Archivos sobre un tablero de ajedrez,
  con su tamaño y dimensiones, y una imagen modificada en la Revisión se
  muestra antes y después lado a lado. Un PDF, o cualquier otro archivo sin
  vista previa, ofrece Abrir en la app predeterminada y Mostrar en la carpeta
  en lugar de un error; en un pull request, un archivo binario enlaza a sí
  mismo en GitHub.
- **La ventana nativa en tu idioma.** Cada pantalla de la ventana nativa se
  lee en portugués, español y chino, y Ajustes › General tiene el idioma de la
  interfaz y el idioma de los prompts, como los tiene la ventana web en
  Apariencia. Antes de que elijas uno, la interfaz sigue el idioma de tu
  sistema. Elegir chino en un equipo sin fuente china ofrece descargar Noto
  Sans SC (unos 25 MB) en la propia carpeta de lich.
- **Japonés.** Ambas ventanas se leen en japonés, y el japonés es un idioma de
  prompts, así que lich habla con tus agentes en él. La ventana nativa dibuja
  japonés, chino y coreano cada uno con la fuente de su región cuando hay una
  instalada (Noto Sans CJK, Yu Gothic, Hiragino, Malgun Gothic y similares),
  de modo que los kanji ya no toman formas chinas, y ofrece descargar Noto
  Sans JP o KR cuando no hay ninguna, igual que hace con Noto Sans SC para el
  chino. El README también está en japonés.
- **Coreano.** Ambas ventanas se leen en coreano, que se elige en Ajustes ›
  Apariencia en la web y en Ajustes › General en la ventana nativa, y el
  coreano es un idioma de prompts: los briefings, los mensajes retransmitidos
  y los avisos llegan a tus agentes en coreano.
- **lich habla ruso.** Elige Русский como idioma de la interfaz, en la ventana
  web en Ajustes › Apariencia o en la ventana nativa en Ajustes › General, y
  como idioma de los prompts, de modo que el briefing, las notas de relevo y
  los traspasos que lich escribe en tus agentes también se lean en ruso. Los
  recuentos siguen las formas singular, paucal y plural del ruso (1 файл,
  2 файла, 5 файлов).
- **Perfiles de opencode en macOS y Windows, y perfiles de Cursor CLI en
  Windows.** Ajustes › Proveedores en la ventana nativa los lista como en
  Linux. Cursor CLI en macOS y Kiro CLI en macOS y Windows siguen sin ellos:
  allí guardan el inicio de sesión en el llavero o en una carpeta que ningún
  ajuste puede mover, así que todos los perfiles compartirían una misma
  cuenta.

### Changed

- **Cada actualización abre la ventana nativa.** El primer inicio de cada
  versión cambia lich a la ventana nativa, recurriendo a la web donde no puede
  arrancar; vuelve a cambiar en Ajustes y lich se queda en la ventana web
  hasta la siguiente versión. La primera vez que se abre la ventana nativa,
  avisa de que es experimental y enlaza a un informe de errores.
- **Un anillo de estado más ligero en la ventana nativa.** El anillo alrededor
  del icono de una tarjeta de sesión se dibuja más fino, para leerse con el
  peso que le da la ventana web.
- **Teclas de atajo más espaciosas en la ventana nativa.** Las teclas de la
  paleta de comandos, la superposición de atajos y Ajustes › Atajos tienen más
  espacio alrededor del texto, que queda centrado.
- **Ajustes en la ventana nativa se abre en Global.** El engranaje de la barra
  superior siempre abre Ajustes en Global en lugar del ámbito en el que lo
  dejaste; la pestaña de un proyecto sigue abriendo sus propios ajustes desde
  Ajustes del proyecto….
- **Una paleta de comandos más tranquila en la ventana nativa.** Cada sesión,
  proyecto y mensaje ocupa una línea, con su ruta después del nombre; la ruta
  de una worktree muestra sus últimas tres partes. Las sesiones que esperan
  por ti van primero y lo indican, y las pestañas de filtro cambian sus
  píldoras por un subrayado. Una sesión cerrada en Historial muestra a la
  derecha su rama y su antigüedad, y una segunda línea solo cuando una
  búsqueda coincidió con su conversación, con las palabras marcadas.

### Fixed

- **Una sesión reiniciada ya no aparece como ocupada para siempre.** Una
  sesión cuyo agente murió a mitad de un turno, o que se cerró a mitad de un
  turno, volvía bajo la misma tarjeta todavía listada como ocupada para las
  demás sesiones: una tarea enviada a ella esperaba un turno que nunca
  terminaría. Ahora arranca sin estado, como una nueva.
- **Codex, Cursor CLI, Antigravity y Crush dejan de preguntar donde ya
  respondiste.** Una sesión en sandbox de cualquiera de ellos se abre
  directamente en su prompt. Cursor CLI y Antigravity ya no vuelven a
  preguntar en una worktree de una carpeta en la que confiaste, ni Antigravity
  en sus subcarpetas, y Crush ya no pide inicializar cada worktree nueva. Una
  sesión de oh-my-pi en sandbox ya no se abre en su asistente de
  configuración.
- **Claude Code deja de preguntar "¿confiar en esta carpeta?" donde ya
  respondiste.** Una carpeta en la que confiaste en tu cuenta principal es de
  confianza en todos los demás perfiles de Claude Code desde su siguiente
  sesión allí, y una sesión de Claude Code en sandbox se abre sin la pregunta.
  Una sesión que lich abrió por ti ya no se queda en esa pantalla, donde Enter
  sale.
- **Una ventana nativa reabierta sabe adónde fue cada sesión.** Una sesión que
  se había movido con `cd`, o dentro de tmux o ssh, volvía mostrando la ruta,
  la rama y los cambios de su checkout hasta su siguiente `cd`; ahora la
  ventana lee dónde estuvo cada una por última vez al conectarse.
- **La ventana nativa escribe los prompts de los agentes en tu idioma de
  prompts.** Los traspasos de pull requests e issues y un lote de comentarios
  de revisión se escribían siempre en inglés.
- **Un cambio de tema durante una reconexión ya no cuelga la ventana nativa.**
  Cambiar el tema, o que el sistema alternara entre claro y oscuro, mientras
  la ventana se reconectaba a un lich reiniciado podía pintar terminales que
  la reconexión acababa de liberar y derribar la ventana.
- **El texto en chino, japonés y coreano ya no congela la ventana nativa en
  Linux.** Una sesión, una Revisión o un pull request llenos de texto CJK
  podían colgar la ventana durante segundos, lo bastante para "no responde", y
  un zoom traía el congelamiento de vuelta.
- **La ventana nativa usa menos memoria.** El terminal de cada sesión y la
  Revisión cargaban su propia copia de las fuentes de reserva, decenas de
  megabytes cada una para una fuente CJK o de emoji: seis sesiones abiertas
  ahora ocupan cerca de la mitad de la memoria que antes. También arranca más
  rápido en un equipo con fuentes CJK instaladas.
- **Reabrir una sesión cuya conversación ya está abierta enfoca esa
  tarjeta.** Una sesión cerrada desde Historial, un cierre deshecho o una
  worktree existente podían volver como una segunda tarjeta sobre una
  conversación que otra tarjeta seguía ejecutando, y las dos escribían en la
  misma transcripción. Historial lista una conversación así una sola vez, por
  su último cierre, y Olvidar elimina todas sus copias.
