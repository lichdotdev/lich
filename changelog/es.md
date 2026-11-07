## [0.86.0] - 2026-11-07

### Added

- La bandeja te dice qué sesiones te necesitan. Su menú lista las sesiones que esperan algo de ti, con lo que preguntan, y aquellas cuyo turno terminado aún no has leído, y después las que trabajan, con la herramienta que ejecutan; al hacer clic en una, la ventana se abre en su tarjeta. Pasar el puntero por el icono las lista todas, sin recortar, donde tu bandeja muestra el tooltip del icono, y en macOS el tooltip de cada entrada trae su línea completa. El icono lleva un punto cuando hay algo en marcha: ámbar para una sesión que espera algo de ti, verde para un turno sin leer, gris mientras una trabaja. Los servicios pasaron a un submenú.
- La bandeja de notas de la página del navegador de una sesión tiene un cuadro para una nota sobre todas ellas, como el lote de comentarios de revisión. Lo que escribes ahí cierra el mensaje que recibe el agente. Enter envía las notas, Shift+Enter empieza una línea nueva y Borrar la vacía junto con el resto.

### Changed

- La ventana gasta aproximadamente una décima parte menos de CPU en cada fotograma que dibuja: ya no calcula qué parte del fotograma cambió cuando de todos modos está dibujando el fotograma entero.
- Mover el puntero sobre la ventana ya no la redibuja en cada movimiento: dibuja solo cuando algo bajo el puntero cambia, y un agente que sigue el ratón (Claude Code lo hace) ya no hace que su salida se dibuje al instante mientras mueves el puntero. Menos energía gastada mientras tienes la mano en el ratón.
- Un enlace en una terminal se subraya, y el puntero se convierte en una mano, solo mientras se mantiene pulsado Ctrl (Cmd en macOS), la tecla con la que un clic lo abre, como en Ghostty. Antes, se subrayaba todo enlace bajo el puntero.

### Fixed

- Escribir o pegar en una terminal desplazada hacia arriba la devuelve al final, donde aparece lo que escribes, como hace la mayoría de las terminales. Antes, el texto llegaba a la sesión mientras la vista seguía en el historial, y había que desplazarse hacia abajo a mano para verlo.
- El texto de la terminal tiene el mismo grosor que el resto del texto de la ventana. Antes, los bordes suaves de cada letra se mezclaban de otra manera, así que el texto claro sobre un tema oscuro se veía más fino de lo debido y el texto oscuro sobre un tema claro se veía más grueso.

## [0.85.1] - 2026-11-05

### Changed

- La nota bajo un lote de comentarios de revisión, en Código, Revisión y Pull requests, ahora es un cuadro que crece a medida que escribes: Shift+Enter empieza una línea nueva y Enter la sigue añadiendo al lote.

### Fixed

- Una variable que el rc de tu shell exporta con un salto de línea en su valor (un certificado, una clave) llega completa a tus sesiones. Antes solo llegaba su primera línea, y una línea posterior escrita como `NAME=value` aparecía como una variable aparte.
- Un archivo marcado como Visto en la pestaña Archivos de la pantalla de Pull requests se abre plegado cuando se vuelve a abrir el pull request, hasta que un nuevo commit lo cambie. Antes volvía abierto.
- Abrir lich de nuevo justo después de cerrarlo, o el reinicio tras una actualización, ya no falla con "lich could not listen on 127.0.0.1:47821" mientras el lich anterior aún detiene los servicios de un proyecto. Al salir, ahora libera el puerto antes de esperarlos.

## [0.85.0] - 2026-11-04

### Added

- El menú de un checkout en la barra lateral, desde un clic derecho o el … de su fila, tiene Cerrar todas las sesiones: todas las sesiones de ese checkout se cierran a la vez, cada una con su propio Deshacer, tras una confirmación cuando alguna está a mitad de un turno, y la última ofrece eliminar el worktree.

### Fixed

- En Linux, la ventana ya no se convierte en píxeles viejos dispersos con controladores gráficos que informan mal de lo que contienen sus búferes, como hace NVIDIA 595 en COSMIC: cada fotograma vuelve a dibujarse completo, como antes de 0.84.0. Dibujar solo lo que cambió, con su menor carga en la tarjeta gráfica, ahora es opcional con `GIO_DAMAGE=on` en el entorno.
- A un lich compilado desde una release con una etiqueta propia en su versión, como `0.83.0+local.3`, Buscar actualizaciones y la comprobación al iniciar le ofrecen la siguiente release. Antes, cualquier build con un `+` en su versión se tomaba por un build de desarrollo y se le decía que estaba en la última versión.

## [0.84.0] - 2026-11-03

### Added

- La pestaña Conversación de un pull request en la pantalla Pull requests muestra la foto de GitHub de cada autor junto a sus revisiones y comentarios, en una columna a la izquierda, como hace GitHub. Los comentarios dentro de un hilo de revisión, en la pestaña Conversación y en la pestaña Archivos, se separan con una línea. Hasta que carga la foto, o si la cuenta no tiene ninguna, en su lugar aparece la inicial del autor.
- Las listas de tareas en las descripciones y los comentarios de pull requests muestran una casilla, marcada o no, en lugar de un `[x]` o un `[ ]` literal.
- Los encabezados de primer nivel en las descripciones y los comentarios de pull requests se subrayan a todo el ancho del texto, como en GitHub.
- La skill de temas del plugin de lich, que escribe, porta o corrige un tema de colores de lich, ahora llega a opencode, oh-my-pi, Crush y Kiro CLI como `lich-theme`; antes solo estaba en Claude Code, Codex, Antigravity y Cursor CLI. En opencode es el comando `/lich-theme`. Llega con la próxima instalación o actualización del plugin desde Ajustes › Proveedores.
- opencode 2 funciona en lich. lich lo distingue de opencode 1 por su versión y lo abre como 2 espera: cada sesión en un servidor propio, de modo que su perfil, su modelo y las herramientas de lich le llegan, con bifurcaciones, Desplegar pegados largos, inicios de sesión de perfil y el medidor de OpenCode Go funcionando igual que en opencode 1. Las conversaciones iniciadas en opencode 2, y las de opencode 1 que este ha continuado, vuelven a mostrar su coste, sus últimas palabras y sus resultados de búsqueda, y aparecen en Historial. El spinner, el anillo, la campana y el título de la tarjeta necesitan la próxima versión del plugin de lich, instalada desde Ajustes › Proveedores, que ahora va adonde opencode 2 lee los plugins y sustituye la copia que instaló un lich anterior. Una sesión que abre lich no está en el servidor compartido en segundo plano de opencode 2, así que `opencode pair` y la app de escritorio no la ven.
- El encabezado Descripción del Resumen de la pantalla Pull requests tiene un menú ⋯, como los comentarios de GitHub: Copiar enlace copia la dirección del pull request, Copiar Markdown su descripción tal como está escrita, y Editar abre la descripción para editarla, lo que antes hacía el botón Editar junto al encabezado.
- Un agente puede gestionar los servicios de un proyecto como lo hace el pie Servicios: listar, crear, editar, eliminar, iniciar, detener, forzar la detención y reiniciar uno, iniciarlos o detenerlos todos, y leer lo que uno ha impreso, en marcha o detenido, con las herramientas MCP `*_service` o `lich service` desde una terminal. Un servicio se nombra por su nombre en el proyecto, el del propio agente salvo que indique otro.

### Changed

- En tu propio pull request, la pantalla Pull requests ya no ofrece Aprobar ni Solicitar cambios, que GitHub rechaza al autor, y el menú del diff ya no ofrece un comentario sobre el pull request. El menú del diff también omite Comentar en el pull request cuando no hay líneas seleccionadas, donde antes lo mostraba desactivado.
- Las descripciones, los comentarios y las vistas previas de Markdown de los pull requests se leen mejor, con texto más brillante, encabezados y enlaces más claros, código en línea resaltado, más espacio entre líneas y bloques, y elementos de lista con salto de línea alineados con su texto.
- La pantalla Pull requests se abre en un índice de los pull requests del proyecto a todo el ancho de la pantalla, y un pull request se abre en su lugar en vez de junto a la lista. Un botón de volver sobre su título regresa al índice como lo dejaste; junto a él, el número abre un menú con los otros pull requests, y anterior y siguiente recorren el filtro del índice. En la pestaña Archivos solo el árbol de archivos queda junto al diff, que gana el ancho que ocupaba la lista. El Resumen pone la descripción junto a una columna con los revisores, las comprobaciones, la posibilidad de fusionar y el checkout donde está la rama; la barra de pestañas termina con el tamaño del pull request. Abrir en una sesión y Fusionar se quedan en el encabezado, con Resolver conflictos o Corregir errores de CI a su lado cuando el pull request lo necesita, y Aprobar, Actualizar y Abrir en GitHub pasan al menú ⋯. La pantalla se alinea con el espaciado de la barra lateral, sus botones tienen el tamaño de los del resto de la ventana, y el código sobre el que trata un hilo de revisión va en un bloque propio, aparte de los comentarios. En el índice, j y k o las flechas mueven, Enter abre y / va al filtro; en un pull request, Escape vuelve y Alt con una flecha pasa al anterior o al siguiente.
- El tema Dark incluido da a la interfaz y a la terminal el mismo fondo carbón neutro, eliminando el tono azul negruzco de la terminal, y usa una paleta de terminal y de sintaxis más brillante de verde, coral, ámbar, azul, violeta y cian. El tema Light usa versiones más oscuras de esos colores, ajustadas para dar contraste sobre su fondo de terminal claro.
- El navegador de una sesión ya no se cierra sin preguntar. Cuando ningún panel lo muestra y nadie lo ha usado en cinco minutos, un aviso pregunta si cerrarlo: Cerrar lo cierra, y Mantener abierto, o descartar el aviso deslizándolo, lo mantiene otros cinco minutos. Sin respuesta, el aviso sigue visible y el navegador abierto, así que una página dejada abierta durante el almuerzo sigue ahí cuando vuelves. Usar el navegador, o que lo use el agente, retira el aviso. Antes, el navegador se cerraba tras cinco minutos lejos de su sesión, y se perdían su página, sus pestañas y su inicio de sesión.
- En la pantalla Pull requests, el botón que envía los hilos de revisión abiertos de un pull request a su sesión está en el encabezado junto a Abrir en una sesión, en todas las pestañas, con el tamaño de los botones que lo rodean. Antes era un botón pequeño en la parte superior de la pestaña Conversación, fuera de vista en cuanto se desplazaba la conversación.
- La franja para redimensionar la barra lateral está en el borde derecho de la propia barra lateral, como la del panel Código y Revisión, en lugar de en el hueco entre la barra lateral y las sesiones.
- La barra lateral es un árbol de sesiones. Una sesión es una línea: su anillo, su nombre, su pull request y sus cambios, y una esquina donde un … bajo el puntero abre el menú de la sesión, con Cerrar incluido. Lo que decían las demás líneas de la tarjeta pasó a su tooltip: la herramienta que ejecuta el turno o lo que espera, durante cuánto tiempo, la ruta y el modelo. Cada checkout es una fila propia, con su rama, su pull request y sus cambios, y de ella cuelgan sus sesiones. Una sesión que abrió otra (una bifurcación, una sesión abierta con `open_session` o `lich open`, una tarjeta de subagente) cuelga de la sesión que la abrió, aunque se ejecute en otra worktree, y una sesión madre pliega a sus hijas, conservando un contador y la marca de espera o de sin leer. Las filas se mueven por su línea: al abrirse una madre, abrirse una sesión o aparecer una worktree o una carpeta, la fila se abre con su nombre ya puesto y la línea baja hasta ella y dibuja su anillo; al plegar o cerrar, se quita el nombre, se borra el anillo y la línea vuelve a subir, y cuando sale la última sesión de una worktree o de una carpeta, la fila de la worktree, o la carpeta, desaparece tras ella. Una carpeta se asienta sobre una bandeja, así que se ve dónde termina. Fijar pasó al menú de la tarjeta, que también copia el comando `lich reply` de una solicitud que la sesión debe responder. Un proyecto sin worktree mantiene una lista plana. La barra lateral contraída muestra el mismo árbol en una columna: el nodo de cada checkout, la línea que atraviesa sus sesiones, el anillo más pequeño de una hija sobre la línea de su madre, y cada carpeta sobre su bandeja, con sus filas yendo y viniendo por la misma línea. Arrastrar una sesión la suelta de su línea, muestra un hueco discontinuo donde se unirá al árbol, y vuelve a dibujar la línea hasta ella cuando cae; la carpeta o worktree en la que se archivaría se ilumina entera. Una madre arrastrada se lleva a sus hijas, y una sesión solo cae donde el árbol la admite, nunca entre una madre y su hija. Una sesión archivada en una carpeta que lleva color toma ese color, de modo que la carpeta lo conserva, ya sea que la arrastres allí, elijas la carpeta en su menú o un agente la archive con `file_session` o `lich file`; una sesión abierta en una carpeta, el trabajador de un subagente o la sesión que lo pidió, también lo lleva. Soltar una sesión sobre una carpeta más abajo en la barra lateral la archiva de nuevo, donde antes volvía a su sitio si una worktree situada encima también la recogía. La barra lateral contraída también arrastra sesiones, y su botón de búsqueda abre el mismo filtro que la barra abierta, que sigue filtrando la barra contraída después de cerrar su panel.
- En el Resumen de la pantalla Pull requests, el encabezado Descripción y su ⋯ se quedan fijos en la parte superior de la columna mientras la descripción se desplaza por debajo, sobre un separador, y el ⋯ queda en el borde derecho de la columna.
- Los marcadores de posición de carga de la pantalla Pull requests coinciden con lo que se carga en su lugar: el índice muestra filas de la altura de las reales con la columna de comprobaciones donde caerá, llenando la pantalla, y un pull request muestra su fila de título con sus botones, la barra de pestañas real y el diseño de la pestaña abierta (la descripción junto a sus datos, los commits, las comprobaciones, los avatares y comentarios de la conversación, o la barra de herramientas, el árbol y las tarjetas de los archivos) en vez de unas pocas barras genéricas. La conversación y el diff muestran los mismos marcadores mientras cargan, en lugar de una línea de texto.

### Fixed

- Una revisión, una respuesta u otra acción sobre un pull request que GitHub rechaza explica el motivo, como una revisión pendiente dejada abierta en github.com o un comentario en una línea fuera del diff. Antes, todo rechazo decía «gh could not complete the request», porque el motivo que enviaba GitHub se descartaba.
- Un comentario de revisión pendiente bajo su línea en la pestaña Archivos de la pantalla Pull requests tiene Editar y Descartar, como en la pestaña Conversación. Antes, solo se podía cambiar o descartar desde la pestaña Conversación.
- En la pestaña Archivos de la pantalla Pull requests, expandir las líneas ocultas entre cambios, previsualizar un archivo Markdown y mostrar una imagen funcionan en un pull request cuya rama no está en checkout en ningún sitio. Fallaban con «Couldn't read» el archivo, porque lich lo buscaba en una carpeta con el nombre de la dirección del pull request.
- La pestaña de un pull request en la pantalla Pull requests se abre en su parte superior. Antes, se abría donde estaba desplazada la pestaña o el pull request anterior, así que la pestaña Conversación podía abrirse al final.
- El código citado en un hilo de revisión de la pantalla Pull requests muestra su sangría en vez de un recuadro por cada tabulación.
- Un campo de archivo en el navegador de la sesión, como la subida de una foto de perfil, abre el selector de archivos del sistema al hacer clic, y admite varios archivos cuando el campo lo permite. Antes, el clic no hacía nada.
- Salir de una página del navegador de la sesión que pregunta antes de irte ya no recarga la página en una máquina ocupada; el navegador va adonde iba.
- Un agente que hace clic en un punto del navegador de la sesión ya no falla tras mover lich a otro monitor, redimensionar el panel del navegador o cambiar su zoom. Sus capturas están ahora en los mismos píxeles que sus clics, que un monitor HiDPI o un zoom de página había hecho hasta 1,5 veces mayores, y un clic en un punto leído de una captura tomada antes de que el panel cambiara de tamaño se rechaza, indicando al agente que tome una nueva.
- lich ya no mantiene ocupada la GPU mientras una sesión trabaja. El spinner de una pestaña de proyecto redibujaba toda la ventana a la frecuencia de refresco del monitor, y con sesiones ocupadas en más de un proyecto lich retenía más de la mitad de la GPU de un portátil, quitándosela a los juegos y a todo lo demás. Cada spinner, el anillo de ocupado de la barra lateral y el latido de un servicio en marcha se redibujan ahora como máximo 24 veces por segundo, y 8 veces por segundo mientras la ventana de lich no tiene el foco, de modo que el anillo de una sesión se sigue viendo girar desde otro monitor. El punto de un servicio en marcha late cada seis segundos en lugar de cada dos y medio: cada latido es una ráfaga de fotogramas, y el ritmo anterior eran ocho fotogramas por segundo todo el día para un punto.
- Mientras una sesión trabaja, la ventana dibuja 24 fotogramas por segundo, la frecuencia del cine, en lugar de 30, y la salida de la terminal de la sesión se dibuja en el siguiente fotograma del anillo de ocupado en vez de pedir fotogramas propios: un agente que emite y un anillo que gira cuestan una sola frecuencia de fotogramas, no la suma de las dos. Lo que escribes se sigue viendo en el momento en que la terminal lo repite.
- La terminal ya no redibuja la fila del cursor en cada fotograma mientras el cursor no se mueve; la fila se recompone cuando el cursor se mueve o cambia su texto.
- En Linux, la ventana dibuja solo lo que cambió desde el último fotograma y avisa de esa área al compositor, en lugar de repintarlo todo 24 veces por segundo mientras una sesión trabaja: un fotograma en el que solo giró el anillo de ocupado dibuja el anillo y la fila del cursor de la terminal, y el trabajo de la tarjeta gráfica en el renderizador de la ventana baja a una quinceava parte. Windows y macOS siguen dibujando fotogramas completos. `GIO_DAMAGE=off` en el entorno dibuja fotogramas completos también en Linux, para un controlador gráfico que informa mal de lo que contienen sus búferes.
- La ventana de lich usa la mitad del procesador que usaba mientras una sesión trabaja. Cada icono, anillo y arco en pantalla se reconstruía a partir de su contorno en cada fotograma; ahora cada uno se construye una vez y se conserva en la tarjeta gráfica.

## [0.83.0] - 2026-10-29

### Added

- La terminal dibuja las imágenes enviadas con el protocolo de gráficos de Kitty e informa a los programas del tamaño de sus celdas en píxeles (`CSI 14 t`, `16 t`, `18 t`). Una TUI que muestra iconos o vistas previas de este modo, como el gestor de archivos `tfm`, ahora las muestra en una sesión de lich igual que en Kitty o Ghostty. Antes, esos programas recurrían a pequeños glifos de texto o no dibujaban nada. Las imágenes enviadas como Sixel o a través de tmux siguen sin dibujarse.
- El navegador de la sesión tiene pestañas. Están en el encabezado del navegador, donde un + abre una nueva; haz clic en una pestaña para cambiar a ella, o en su × para cerrarla. Lo que una página abre en una pestaña o ventana nueva se suma a ellas, al frente, y cerrarla devuelve la página que la abrió, tal como estaba. El agente lista y cambia de pestaña con las nuevas herramientas MCP `browser_tabs` y `browser_tab` (o `lich browser tabs` y `lich browser tab`), y su cursor se mueve a la pestaña en la que actúa.
- `lich insert --focused` pega en la sesión cuya tarjeta está a la vista en la ventana de lich: la tarjeta activa de la pestaña de proyecto que está al frente. Una acción del gestor de archivos o un editor puede enviar una ruta al agente que estás mirando sin deducirlo de la carpeta del archivo. `lich sessions --json`, `lich whoami --json` y la herramienta MCP `list_sessions` marcan esa sesión con `"focused": true`. Con la ventana cerrada, ninguna sesión se marca y `--focused` lo indica.

### Changed

- Lo que una página del navegador de la sesión abre en una pestaña o ventana nueva ya no reemplaza la página que la abrió. Antes, la dirección de la pestaña nueva se cargaba sobre esa página, así que se perdían un inicio de sesión o cualquier otra cosa que la página guardaba en su script, y fallaba un flujo que abre una pestaña y espera a que termine.

### Fixed

- Cerrar una sesión cuyo navegador estaba abierto a su lado, con su terminal enfocada, ya no cierra la ventana de lich.

## [0.82.0] - 2026-10-29

### Added

- `lich sessions --json`, `lich whoami --json` y la herramienta MCP `list_sessions` incluyen la línea que la tarjeta de una sesión muestra bajo su etiqueta: `tool` y `detail` mientras está ocupada, la herramienta que ejecuta y sobre qué actúa, y `reason` mientras espera tu respuesta. Un script o un widget de escritorio puede mostrar lo que hace cada agente sin abrir la ventana.
- El navegador de una sesión abre un archivo local del checkout de la sesión. Escribe su ruta en el campo de dirección, o el agente pasa la ruta o una URL `file:` a `browser_open`, y lich sirve el checkout por http en 127.0.0.1, en un puerto propio de cada worktree (el `LICH_WORKTREE_PORT` del checkout más 1000), de modo que los scripts de módulo, `fetch` y los recursos relativos funcionan y cada worktree conserva su propio almacenamiento. Una ruta fuera del checkout se rechaza. Antes, un archivo local no se podía abrir en absoluto, y previsualizarlo significaba iniciar un servidor estático a mano.
- La barra de herramientas del navegador de una sesión tiene un botón de zoom. Abre un pequeño menú que reduce y aumenta el zoom de la página de 10 en 10% y lo restablece al 100%, y cada control indica su atajo de teclado. Antes, el zoom solo se podía cambiar desde el teclado.

### Fixed

- Continuar en conserva el navegador de la sesión. Mover una sesión a otro perfil cerraba su navegador, y su página y sus cookies desaparecían cuando la sesión volvía a iniciarse.
- El navegador de la sesión sigue el ritmo de una página ocupada. En una página que anima o ejecuta mucho JavaScript, la pestaña del navegador se atrasaba segundos respecto a la página y el resto de la ventana esperaba con ella; ahora salta al fotograma más reciente.
- El navegador de la sesión se ve nítido en una pantalla HiDPI. Su pestaña mostraba la página a 1x, borrosa, en cualquier pantalla; ahora la dibuja hasta a 1,5x.
- Las notas del navegador de la sesión, y Corregir errores de CI y Resolver conflictos de la pantalla Pull requests, llegan a una sesión de Claude Code que detuviste con Esc. Claude Code rechazaba el mensaje hasta que alguien escribía en la sesión, y la bandeja decía «its mod refused it»; ahora lich escribe el mensaje en el prompt.

## [0.81.0] - 2026-10-28

> [!IMPORTANT]
> **Quien tiene un agente trabajando en una página web: señala la página y dile qué cambiar.** La mira de la barra de
> herramientas del navegador de la sesión deja notas sobre los elementos que eliges y las envía al agente en un solo mensaje.

### Added

- La mira de la barra de herramientas del navegador de una sesión te permite dejar notas en la página para el agente. El elemento bajo el puntero se resalta y se nombra, con el contorno deslizándose de un elemento al siguiente al mover el puntero; un clic abre una nota a su lado, y cada nota fija su elemento con un número, siguiéndolo mientras la página se desplaza. Las notas esperan en una bandeja flotante sobre la parte inferior de la página, que nunca cambia de tamaño bajo tus manos. Al pasar el ratón por una nota ahí, su elemento se ilumina, y un clic en ella o en su marcador desplaza el elemento a la vista y edita la nota. Un elemento que una lista con desplazamiento, una barra lateral o un diálogo recorta se resalta solo donde se ve, y uno que queda oculto no deja marcador sobre lo que tiene al lado. Enviar (o Ctrl+Enter) las escribe en la sesión como un solo mensaje: qué elemento, un selector que acepta el `browser_click` del agente, el componente de React que lo dibujó en una compilación de desarrollo, y tu nota. Ctrl+Shift+E (Cmd+Shift+E en macOS) activa y desactiva la mira, y Esc sale de ella. Un envío que la sesión rechaza conserva las notas y explica el motivo en la bandeja.
- El navegador de una sesión descarga archivos. Una descarga que inicia la página, desde un enlace, un botón, una pestaña nueva o un script, llega a tu carpeta Descargas con el nombre que le da la página, añadiendo ` (1)` en lugar de sobrescribir un archivo que ya esté ahí. El agente también puede abrir directamente la URL de un archivo. El clic, la pulsación de tecla o la apertura del agente responde con la ruta y el tamaño del archivo, y la ventana muestra un aviso con el nombre del archivo y un botón que abre su carpeta. Antes, un botón de descarga en el navegador de la sesión no hacía nada.
- El agente puede esperar a que cambie la página de su navegador en lugar de volver a leerla en un bucle. `browser_info` acepta `wait_for` (texto que debería aparecer) y `wait_gone` (texto que debería desaparecer), buscados en cualquier parte de la página, y responde en cuanto la página lo muestra, o tras 20 segundos con la página tal como está y una línea que indica que la espera agotó el tiempo.
- El agente se entera cuando falla la página del navegador de su sesión. Cada página que lee, en la que hace clic o pulsa una tecla responde con los errores de consola desde su última lectura, una vez cada uno: llamadas a `console.error`, excepciones no capturadas y recursos que no se cargaron, recortados a una línea corta, los primeros cinco y cuántos más hay. Las advertencias y otros mensajes quedan fuera.
- Un agente puede cerrar un proyecto, con `close_project` o `lich close-project`: la pestaña sale de la ventana y sus sesiones se detienen, conservadas para reabrirlas como cuando cierras la pestaña. El proyecto en el que se ejecuta el agente y el proyecto de inicio se rechazan.
- Tú y el agente respondéis a los diálogos de JavaScript de la página en el navegador de una sesión. Un alert, un confirm, un prompt o un "¿Salir de esta página?" que abre la página aparece como una tarjeta sobre la parte superior de la página en la pestaña del navegador de la sesión, con el nombre del sitio que lo abrió, sus botones y, para un prompt, un campo. Lo respondes ahí, o lo hace el agente con `browser_dialog`, y su cursor pulsa el botón de la tarjeta. El clic del agente que abrió un diálogo responde de inmediato con el diálogo incluido, y el agente se entera de uno que respondiste tú. Antes, un botón que mostraba un alert congelaba la página: el clic del agente y cada lectura posterior esperaban treinta segundos y fallaban.
- Ajustes › General tiene un interruptor Copiar al seleccionar para la terminal. Activado por defecto, copia una selección en cuanto la haces; desactivado, cópiala con Ctrl+Shift+C (Cmd+C en macOS).
- El navegador de una sesión hace zoom de su página por su cuenta, aparte del zoom de la interfaz: Ctrl+- y Ctrl+= (Cmd en macOS) alejan y acercan un 10%, del 50% al 200%, y Ctrl+0 vuelve al 100%. Una página alejada se distribuye más ancha, como en un navegador de escritorio, lo que hace utilizable una pantalla pequeña sin encoger el resto de la ventana. El zoom se aplica al navegador de todas las sesiones, aparece en la barra de herramientas mientras no sea 100% y se conserva para el próximo inicio.
- Ajustes › General › Ventana › Botones elige si se muestran los botones de minimizar, maximizar y cerrar de la ventana: Seguir el sistema, Mostrar siempre u Ocultar. Seguir el sistema es como se comportaba lich antes, y en Linux ahora también sigue un cambio en la disposición de botones del escritorio mientras lich está abierto, en lugar de leerla solo al iniciar. Mostrar siempre dibuja los botones incluso donde el gestor de ventanas decora la ventana o la coloca en mosaico sin barra de título, como hace Hyprland. Ocultar deja la ventana sin ellos, incluidos los semáforos en macOS.

### Changed

- La cuenta de GitHub de un proyecto (Ajustes › Control de versiones) ahora llega también a sus sesiones: `gh` ejecutado por el agente o en una tarjeta de shell responde como esa cuenta en lugar de la que gh tenga activa, desde el próximo inicio de la sesión. Dentro de esas sesiones `gh auth switch` ya no cambia la cuenta.
- La ventana usa menos memoria con muchas sesiones abiertas. Una terminal fuera de pantalla libera los píxeles que dibujó, unos pocos megabytes cada una, y los vuelve a dibujar en un fotograma cuando regresa.
- El recolector de basura de la ventana ahora corre a 50 en lugar del 100 por defecto de Go, lo que en una prueba con 15 sesiones recortó cerca de una décima parte de su memoria residente, el margen que el sistema enviaría a swap. `GOGC` sigue teniendo prioridad.
- Las herramientas del navegador responden al agente en el idioma del prompt de la sesión: la leyenda de la lista de elementos, la nota de que hay más elementos más abajo, las marcas de relleno y de activado, una descarga que sigue en curso, una espera que agotó el tiempo, el recuento de errores de consola adicionales y lo que un diálogo abierto le pide. Las etiquetas de línea (`url:`, `text:`, `downloaded:`, `console error:`, `dialog open:` y el resto) se mantienen como están en todos los idiomas.
- El navegador de una sesión que ninguna pestaña muestra distribuye su página con el tamaño de la ventana de lich, al zoom del navegador, en lugar de los 800×600 de Chrome, así que el agente lee y captura la página como la muestra un navegador de escritorio. Cerrar la pestaña del navegador devuelve la página a ese tamaño en lugar de dejarla con el de la pestaña.
- Resolver conflictos en la pantalla Pull requests nombra los archivos en conflicto en el prompt que envía a la sesión, como Corregir errores de CI nombra las comprobaciones que fallan. Si se pulsa antes de conocer los archivos, envía el prompt sin ellos, como antes.

### Fixed

- En Windows, la ventana se llama `lich` en la barra de tareas y en Alt-Tab, en lugar de `lich (native slice)`.
- Un proyecto cuya cuenta de GitHub vive en un servidor de GitHub Enterprise obtiene esa cuenta en la pantalla Pull requests, en las insignias de pull request y en una sesión en sandbox a la que se le concede su token. El token salía bajo una variable que gh ignora para un host empresarial, así que gh respondía allí como su cuenta activa.
- El clic, la escritura o la pulsación de tecla por índice del navegador de una sesión llega al elemento al que la última lectura de página del agente dio ese índice. Tras un desplazamiento y una segunda lectura, un elemento que la primera lectura había listado con el mismo índice, ahora fuera de vista, podía recibir el clic en su lugar, sin ningún error.
- En Windows, las sesiones que arrancan a la vez en un mismo perfil ya no fallan de vez en cuando con `Access is denied` mientras lich escribe los archivos de ese perfil.

## [0.80.0] - 2026-10-27

> [!IMPORTANT]
> **Quien tiene un agente trabajando en una página web: ahora controla un navegador junto a la sesión.** Abre
> Navegador en el menú de la tarjeta de la sesión y mira el cursor del agente hacer clic, escribir y desplazarse en la página, mientras tú también la usas.

### Added

- El menú de una tarjeta de sesión abre un navegador para esa sesión, encima de Abrir en, en una pestaña junto a la sesión: su terminal a un lado y la página al otro, con el divisor, el giro y la ampliación de una división. El agente de la sesión maneja la misma página: abrir una URL, leerla, hacer clic, escribir, pulsar una tecla, ir atrás y adelante, desplazarse y capturar la pantalla, desde las herramientas `browser_*` o desde `lich browser`. El cursor propio del agente muestra dónde actúa: la flecha de lich, en el color de tarjeta de la sesión o en el del tema, con una insignia que nombra al proveedor, se desliza hasta cada clic, campo y desplazamiento de la página y hasta atrás, adelante, recargar y la dirección en la barra de herramientas del panel, reproduciendo por turnos incluso las llamadas en paralelo de un agente. Tú haces clic, escribes y copias y pegas con tu propio portapapeles en la página mientras tanto. Cada clic y tecla que envía el agente responde con la página que dejó, en forma compacta, de modo que una sesión larga gasta cerca de la mitad de los tokens que gastaría leyendo la página de vuelta como JSON. Cuando el agente abre un navegador por su cuenta, un aviso en la esquina inferior derecha ofrece mostrarlo. Abrir en una ventana mueve la página a una ventana de Chromium propia, con sus cookies y todo. Cerrar la tarjeta cierra el navegador. El navegador es el Chrome, Edge, Chromium, Brave, Vivaldi o Helium que ya tengas instalado.
- El pie de ventana muestra qué parte del último turno se leyó de vuelta de la caché del prompt, con una barra que divide la entrada del turno en en caché, escrita en la caché y sin caché. Un turno posterior a la expiración de la caché marca 0% en ámbar, que es el turno facturado a tarifa completa. Aparece en el tooltip de la ventana de contexto en Claude Code y Codex, y en el tooltip de costo en oh-my-pi y opencode.

### Changed

- Una sesión de opencode recibe el servidor MCP de lich al iniciarse, como Claude Code y Codex, así que lista todas las herramientas de lich, incluidas las `browser_*`, bajo el servidor `lich` en la lista MCP de opencode. Tus propios servidores y cualquier `OPENCODE_CONFIG_CONTENT` que exportes se conservan.
- Un agente puede cambiar el modelo o el esfuerzo de razonamiento de la sesión en la que se ejecuta mediante `control_session`, y cerrar esa sesión cuando se lo pidas (`self` en `close_session`, `lich close --self`). Antes ambas cosas se rechazaban de plano.

### Fixed

- Abrir, recargar, atrás y adelante del navegador de una sesión regresan en cuanto la página es interactiva, en vez de esperar a que terminen todas las imágenes y rastreadores. Esa espera retenía la siguiente acción del agente tanto como tardara el evento load, hasta 30 segundos, cuando la página ya estaba en pantalla. Una captura de pantalla es el viewport que se está viendo.
- El costo de una sesión de Codex ya no cobra dos veces los tokens escritos en la caché del prompt. Se tarificaban a la tarifa de escritura en caché y otra vez como entrada sin caché.
- La rueda del ratón en una terminal se desplaza con suavidad y tan lejos como en otras terminales. Una muesca mueve tres líneas del historial del shell, o de un programa a pantalla completa como `less`, suavizadas a lo largo de unos fotogramas en vez de saltar unas ocho líneas de golpe. Un programa que toma el ratón, como nvim, Claude Code o Codex, recibe un paso de rueda por muesca y se desplaza su propia distancia, cuando antes recibía unos ocho y recorría la conversación a toda velocidad.

## [0.79.0] - 2026-10-24

> [!IMPORTANT]
> **Quien deja un servidor de desarrollo, un contenedor o un túnel abierto junto a sus agentes: ahora lich los
> ejecuta.** Guárdalos una vez en Ajustes › Servicios del proyecto e inícialos, detenlos y léelos al final de la barra lateral.

### Added

- En macOS, lich tiene un elemento en la barra de menús: un lich con la ventana cerrada muestra allí que sigue ejecutando sus sesiones, con Mostrar lich, el número de sesiones en marcha y Salir de lich, como hace la bandeja en Linux y Windows.
- **Servicios: comandos que un proyecto mantiene en marcha, iniciados por lich.** Guarda una vez un contenedor, un túnel o un servidor de desarrollo en Ajustes › Servicios de un proyecto, e inícialo desde la fila Servicios al final de la barra lateral, encima del perfil. Cerrada, la fila indica cuántos están en marcha (2/3); abierta, lista cada servicio con cuánto lleva en marcha o cómo terminó, y Iniciar, Detener y Reiniciar al pasar el ratón. Un clic abre su consola, donde acepta entrada y Ctrl+C lo detiene. Un servicio no es una terminal: cuando su comando termina, se ha detenido, su última salida sigue legible e Iniciar lo ejecuta de nuevo. Marca Iniciar con lich y se inicia cada vez que lich arranca con su proyecto abierto. Los servicios guardados en Ajustes › Servicios con Global seleccionado se ejecutan en tu carpeta personal y aparecen en la pantalla de inicio de lich, el lugar para un `lazydocker` que mantienes abierto sea cual sea el proyecto. Un servicio vive tanto como la pestaña de su proyecto: cerrar la pestaña lo detiene, y reabrir el proyecto inicia los guardados para iniciarse con lich. Cerrar la ventana los deja en marcha, y la bandeja los lista con Mostrar salida, Reiniciar y Detener; salir de lich los detiene, y el diálogo de cierre los cuenta. Con la barra lateral contraída, el botón Servicios del riel abre la misma lista.

- **Envía los hilos de revisión de un pull request a su sesión.** En la pantalla Pull requests, un hilo de revisión abierto tiene Enviar a la sesión junto a Responder y Resolver, en la pestaña Conversación y en el diff, y la pestaña Conversación puede enviar todos los hilos abiertos a la vez. Los hilos van como un solo prompt a la sesión en el checkout de la rama, que lich abre cuando no hay ninguna, con el archivo, las líneas, el enlace y cada comentario de cada hilo. Pull requests sigue en pantalla, y un aviso con Abrir indica cuándo la sesión los recibió. lich responde entonces a cada hilo en GitHub con "Fixing this. It'll be in the next commit." y lo resuelve cuando el pull request se vuelve a enviar con push, respondiendo con el commit en el que quedó; hasta entonces el hilo muestra Enviado · esperando un push. Cualquier push lo resuelve, incluso uno que no lo tocó.

### Changed

- Un pull request fusionado fuera de lich, en github.com, con `gh` en una terminal o por un agente, ahora se anuncia a los agentes del checkout de su rama como uno fusionado desde Pull requests, en el minuto siguiente al merge. Antes seguían creyendo que estaba abierto. Lo oyen los agentes que se ejecutan allí cuando lich lo ve abierto y luego fusionado; uno aparcado en ese momento, o un pull request fusionado antes del último arranque de lich, no.
- Resolver conflictos y Corregir errores de CI en Pull requests ya no te llevan a la sesión a la que entregan el pull request. Pull requests sigue en pantalla para que pases al siguiente pull request, la sesión se inicia en la barra lateral, y un aviso indica cuándo el agente recibió la tarea, con Abrir para ir a ella. Una sesión cuyo inicio necesita antes tu respuesta, como si reanudar su última conversación, sí aparece. Abrir en una sesión sigue yendo a la sesión.
- Los filtros rápidos sobre la lista de pull requests en Pull requests (Todos, Listos, Borradores, Con fallos) son más ligeros: texto pequeño con el seleccionado sombreado, como en las pestañas del dock, y comparten el ancho de la lista. Un filtro sin nada está atenuado y el recuento de Con fallos se vuelve rojo cuando falla una comprobación. Ya no sobrepasan el borde de la lista con una fuente monoespaciada en la interfaz.

### Fixed

- La insignia de pull request en una tarjeta de sesión, su elemento de menú Pull request y el botón de pull request del pie de ventana ya no abren el pull request equivocado después de que el shell de la sesión pasara a otro repositorio. La insignia muestra el pull request de ese repositorio, y el clic buscaba su número entre los del propio proyecto en Pull requests; ahora abre el pull request en GitHub.
- La barra lateral contraída muestra el perfil en el que nacen las sesiones nuevas, al final del riel, y abre el mismo menú que el pie de la barra lateral abierta. Antes no mostraba ningún perfil.
- Una pestaña de la barra superior ya no puede quedarse levantada, dibujada con el color de acento como si se estuviera arrastrando, sin que ninguna pestaña volviera a moverse hasta reiniciar lich. Una pestaña enfocada con un clic y luego accionada con Espacio o Enter se recogía para reordenarla con el teclado, y cuando un clic en otro lugar se llevaba el teclado no podía soltarse ni cancelarse; ahora vuelve a su sitio al perder el foco. Una pestaña de diseño renombrada en mitad de un arrastre ya no se queda atascada de la misma manera.
- Una tarjeta de sesión ya no crece y se encoge cuando su agente inicia y termina una herramienta, espera tu respuesta o compacta, empujando hacia arriba y hacia abajo todas las tarjetas que tiene debajo. Esa línea ahora ocupa el lugar de la ruta de la tarjeta mientras se muestra, y la ruta vuelve cuando se va.
- La ventana nativa usa menos memoria para sus fuentes. Un archivo de fuente pedido más de una vez, como la interfaz y la terminal con la misma familia o la cara negrita que también hace de seminegrita, se analizaba de nuevo cada vez, unos 30MB por cara de una Nerd Font; ahora se analiza una vez y se comparte mientras se usa. Una interfaz en un idioma distinto del chino, japonés o coreano ya no carga una fuente CJK al arrancar, decenas de megabytes: lo hace con el primer texto CJK en pantalla, dibujado mientras tanto con una fuente del sistema. Con una Nerd Font para la interfaz, la terminal y el editor y sin CJK en pantalla, las fuentes ocupan 119MB donde antes ocupaban 246MB.
- En macOS, abrir lich desde el Dock, Launchpad, Spotlight o Finder mientras se ejecuta con la ventana cerrada vuelve a abrir la ventana. También un lich iniciado al arrancar la sesión con `--no-window`, donde el clic no hacía nada. Cerrar la sesión de usuario ahora sale de lich como lo hace `lich quit`, así que el siguiente arranque no lo toma por una caída.

## [0.78.1] - 2026-10-23

### Changed

- Mientras una sesión trabaja, la ventana redibuja su spinner de ocupado a 30 fotogramas por segundo en vez de a la tasa de la pantalla, una tarjeta construye su menú de clic derecho solo mientras ese menú está abierto, y un diálogo cerrado no construye nada.

### Fixed

- La rama, los cambios y el pull request en las tarjetas y en el pie de ventana se ponen al día en menos de un segundo tras un commit, checkout o edición hechos fuera de un turno, cuando antes podían tardar hasta tres segundos, y al seleccionar una tarjeta se lee su checkout al instante en lugar de mostrar lo que encontró la última lectura.
- Ocultar la lista de pull requests en la pantalla Pull requests la quita al instante, cuando antes la lista seguía en pantalla hasta que se movía el ratón o Pull requests volvía a consultar GitHub. Mostrarla otra vez tampoco espera ya un fotograma.
- Las sesiones de Kiro CLI siguen `KIRO_HOME`. Con ella definida, lich leía las conversaciones, el coste y el contexto de Kiro en `~/.kiro` mientras Kiro los guardaba en `KIRO_HOME`, así que no mostraban nada e Historial no listaba ninguna; el agente que lleva los hooks de lich se instalaba donde Kiro nunca miraba, y una sesión en sandbox no recibía el directorio en el que escribe.
- Abrir o cerrar el dock ya no deja la terminal de la sesión cortada en su borde por un instante cuando el dock se detiene. La terminal toma su nuevo ancho en cuanto el dock empieza a moverse, así que el agente redibuja mientras el dock se desliza, cuando antes la terminal esperaba una décima de segundo tras detenerse el dock y luego daba un salto. Las terminales de un diseño dividido se ajustan en el momento en que el dock se detiene.
- Expandir la barra lateral ya no deja la terminal de la sesión con su ancho anterior por un instante cuando la barra ya está abierta. La terminal toma su nuevo ancho en cuanto la barra empieza a moverse, como con el dock, cuando antes esperaba una décima de segundo tras detenerse la barra y luego daba un salto; al contraerla también se reajusta al instante. Las terminales de un diseño dividido se ajustan en el momento en que la barra lateral se detiene.
- Reabrir la ventana ya no escribe texto suelto en una sesión. Reconstruir una terminal a partir de su salida reciente volvía a responder las consultas de terminal que contenía, como las que envía nvim al arrancar, y las respuestas caían en la entrada de la sesión: en un shell como un `command not found`, en un agente como texto en su prompt.
- La terminal de una sesión abierta mientras la sesión imprime ya no pierde lo que imprimió en ese momento. El backend ahora entrega a la ventana la salida reciente de la sesión en línea con su salida en vivo, así que nada entre ambas se pierde ni se muestra dos veces.
- En Linux, cambiar el tamaño de fuente de la terminal y abrir una tarjeta por primera vez se traban menos. Cada terminal volvía a buscar su fuente en fontconfig cada vez, cuatro búsquedas de unos 10 ms cada una dentro de un mismo fotograma; lich ahora recuerda la respuesta durante el resto de su ejecución.
- Cambiar el tamaño de fuente de la terminal ya no recarga la fuente de cada terminal abierta: cada una conserva la fuente que tiene y solo la dibuja al nuevo tamaño.
- Una sesión que imprime mucha salida cuesta menos al backend. Cada byte de camino a la ventana se copiaba unas siete veces a memoria que el recolector de basura tenía que reclamar después; ahora se copia una vez.
- Un comentario de bloque que se abre por encima de un tramo plegado de un diff, en Revisión y en la pantalla Pull requests, ya no colorea como comentario el código posterior a ese tramo. Su `*/` de cierre caía en las líneas ocultas, así que cada hunk de abajo se veía como comentario hasta algún `*/` posterior, y solo expandir el tramo lo arreglaba. Las líneas de un comentario cortado por un tramo plegado, por encima de él, aún pueden verse como código hasta que se expande.

## [0.78.0] - 2026-10-22

### Added

- Los comentarios se pueden editar y eliminar desde la pantalla Pull requests. El … junto a un comentario de un hilo de revisión (en la pestaña Conversación y en el diff) o a un comentario del pull request ofrece Editar, que lo reescribe en el sitio en GitHub, y Eliminar, que pregunta antes. El resumen de una revisión enviada también se puede editar; su veredicto se mantiene. El … solo aparece en lo que GitHub deja cambiar a tu cuenta.
- Al elegir una fuente monoespaciada para la interfaz en Ajustes › Apariencia, lich pregunta si la terminal y el editor también deben usarla. Usar en todo aplica ambas; Solo la interfaz las deja como estaban. Una fuente proporcional o Sistema no pregunta nada.
- Un comentario en espera en el lote de revisión se puede reformular antes de enviarlo: su lápiz lo abre en un diálogo, y se guarda con Guardar o Ctrl+Enter. Un comentario de línea en la revisión pendiente de un pull request tiene un botón Editar junto a Descartar que lo abre en un cuadro, y se guarda con Guardar o Ctrl+Enter.

### Removed

- Se eliminó el aviso de que la ventana nativa es experimental, que se mostraba la primera vez que se abría: la ventana nativa es la única ventana de lich.

### Fixed

- Los comentarios para la sesión escritos en la pestaña Archivos de un pull request en la pantalla Pull requests van a una sesión en el checkout de la rama de ese pull request, cuando antes Enviar los escribía en la sesión que estuviera abierta antes de entrar a Pull requests, revisando otra rama. Si no hay ninguna sesión allí, Enviar avisa que abras una y conserva los comentarios; Abrir en una sesión la inicia. Una referencia de línea enviada desde esa pestaña sigue el mismo camino.
- Ir a la sesión en un pull request de la pantalla Pull requests abre la sesión en la que estás cuando está en el checkout del pull request, cuando antes siempre abría la primera sesión de allí.
- En Windows con un escalado de pantalla superior al 100 %, un arrastre vuelve a seguir al puntero: un elemento arrastrado en Ajustes › Pie de ventana o una selección arrastrada en la terminal quedaban lejos del cursor, escalados como si la pantalla estuviera al 100 %.
- Los comentarios de revisión enviados a una sesión se conservan hasta que la sesión los recibe. Una sesión que rechazó el lote (por ejemplo, una que se había detenido) mostraba "No se pudieron enviar los comentarios" cuando los comentarios ya habían desaparecido; ahora siguen en el lote para volver a enviarlos. Del mismo modo, enviar la revisión de un pull request en la pantalla Pull requests descarta solo los comentarios que incluía, cuando antes un comentario escrito mientras la revisión estaba en camino se perdía con ellos.
- En Windows, la ventana lleva el icono de lich y se abre bajo el botón de la barra de tareas de lich, cuando antes mostraba un icono genérico y, lanzada desde un lich anclado a la barra de tareas, se abría como un botón aparte junto al anclado.
- Los clics en toda la ventana muestran su resultado al instante, cuando antes muchos esperaban a que se moviera el ratón: diálogos que se abren y se cierran, los botones de cierre del dock y de Ajustes, abrir un archivo desde la pestaña Código, los interruptores de la barra de búsqueda, los filtros y la conversación de la pantalla Pull requests, el filtro de la barra lateral, y las secciones, interruptores, escaleras y selectores de Ajustes. Un control segmentado ya no muestra dos opciones pulsadas justo después de un clic.
- Un clic en una carpeta del árbol de archivos de la pestaña Código, o del árbol de archivos de un pull request, la abre o la pliega al instante, cuando antes esperaba a que se moviera el ratón. La pestaña Código también construye en milisegundos el árbol de un checkout con decenas de miles de archivos, cuando antes tardaba segundos, y el resaltado de una fila sigue al puntero sin quedarse un evento atrás.
- Reiniciar lich tras una actualización abre la ventana de la nueva versión, cuando antes la ventana antigua seguía abierta sobre el nuevo lich y una sesión podía fallar al iniciarse con "want 8 arguments, got 9" hasta que se cerraba y volvía a abrir lich. La ventana se cierra y vuelve con el reinicio; un lich que se ejecuta sin ventana sigue sin ella. Esto surte efecto desde la siguiente actualización tras esta versión.
- Ctrl+F en un archivo abierto en la pestaña Código abre la barra de búsqueda, y las teclas escritas allí llegan a ella, cuando antes ambas iban a la terminal de la sesión detrás de la pestaña: la terminal le quitaba el teclado al código, a la barra de búsqueda y a la lista de todas las coincidencias en cada fotograma. Un clic en la terminal ahora le entrega el teclado, así que seleccionar código para copiarlo ya no te deja escribiendo en ninguna parte.
- Un pull request en la lista de Pull requests ya no recorta su veredicto de revisión cuando el nombre del autor es largo: el nombre se acorta, y la carpeta del checkout al final de la línea solo aparece cuando cabe entera, en lugar de como medio icono.
- Un pull request en la lista de Pull requests cuyos hilos de revisión están todos resueltos muestra su "Cambios solicitados" en ámbar con una marca de resuelto, como ya hacía su encabezado, en lugar del rojo de uno que sigue esperando a su autor. GitHub mantiene el veredicto hasta que el revisor aprueba o la revisión se descarta. El "Borrador" de un borrador ahora es una insignia gris con borde, así que ya no se lee como parte de un veredicto ámbar a su lado.

## [0.77.0] - 2026-10-21

### Added

- La pantalla Pull requests muestra una imagen modificada en la pestaña Archivos de un pull request, con el antes y el después lado a lado, como hace la pestaña Revisión, cuando antes decía solo "Archivo binario". Un archivo Markdown editado ahí también tiene el botón de vista previa, cuando antes solo lo tenía uno añadido. lich lee el archivo de tu clon cuando contiene el commit y de GitHub en caso contrario.
- `lich mcp` responde a los clientes MCP en la revisión de protocolo 2026-07-28, que indican la versión en cada petición en lugar de abrir con `initialize`, y atiende su `server/discover`. Los clientes que abren con `initialize` reciben la misma respuesta de antes.
- La tarjeta de Cursor CLI toma el nombre que Cursor da a la conversación, una vez que termina el primer turno, cuando antes conservaba el nombre que le dio lich.

### Changed

- Un merge en la pantalla Pull requests avisa también a la sesión que abrió la sesión de la rama fusionada, se ejecute donde se ejecute, como ya avisaba a las sesiones del checkout fusionado. Una sesión que reparte trabajo se entera de que su pull request se fusionó sin que tú se lo digas.

### Fixed

- Resolver conflictos, Corregir errores de CI y una worktree iniciada desde un issue envían su mensaje a una nueva sesión de Claude Code en cuanto está en su prompt, cuando antes podía quedarse sin enviar hasta un minuto, hasta que pulsabas Enter. lich leía las respuestas del propio terminal a las preguntas de inicio de Claude Code como si fueras tú escribiendo en ese prompt, y retenía el mensaje por ti.
- Un archivo Markdown largo, como el CHANGELOG.md de lich, se previsualiza en las pestañas Código y Revisión sin congelar la ventana: la vista previa dibuja solo la parte en pantalla, cuando antes maquetaba el archivo entero en cada fotograma.
- Una vista previa de Markdown mantiene separadas las palabras donde el origen parte una línea, cuando antes quedaban pegadas, y una tabla ya no estrecha una columna angosta hasta romper las palabras: cada columna es al menos tan ancha como su palabra más larga.
- La tarjeta de Claude Code vuelve a tomar el título que Claude Code da a la conversación, cuando antes conservaba el nombre que le dio lich. lich ahora inicia una nueva sesión de Claude Code sin `--name`, lo que impedía a Claude Code ponerle título. La sesión elige su propio nombre en la lista de agentes de Claude Code, distinto del de la tarjeta, y `lich sessions`, `lich whoami` y `list_sessions` no muestran un segundo nombre para ella hasta que un `/rename` le da uno.

## [0.76.0] - 2026-10-21

### Added

- Un archivo Markdown abierto en la pestaña Código tiene el mismo botón de vista previa que en la pestaña Revisión, junto a la marca de solo lectura, y muestra el archivo renderizado, con tablas y enlaces, en lugar de su código fuente. Púlsalo de nuevo para volver al código fuente. Cualquier otro archivo se abre como código fuente.

### Changed

- Enviar en un lote de comentarios de revisión, en las pestañas Revisión y Código y en la pantalla Pull requests, los envía a la sesión como su prompt, cuando antes esperaban en el prompt a que pulsaras Enter. Una sesión de shell conserva los comentarios, ya que allí un Enter los ejecutaría como comando.

### Fixed

- Las teclas muertas componen en el terminal de una sesión: en macOS, ~ y luego a escribe ã, y ' y luego c escribe ç, cuando antes el acento llegaba solo antes de la letra. El texto que un método de entrada aún está componiendo, como el de un IME japonés o chino, tampoco llega ya al programa antes de confirmarse.
- Tras cambiar de modelo en una sesión de Claude Code con `/model`, el pie y el tooltip de la tarjeta de la sesión nombran el nuevo modelo en cuanto termina su primer turno. Antes conservaban el anterior hasta un turno posterior o una reconexión, porque lich leía el transcript antes de que Claude Code hubiera escrito la última línea de ese turno.
- En macOS, los botones de cerrar, minimizar y zoom se mantienen centrados en la barra de título. macOS los devolvía a la parte superior de la barra al abrirse la ventana, al cambiar a otra app y volver, y cuando el sistema alternaba entre claro y oscuro, hasta que se redimensionaba la ventana.

## [0.75.0] - 2026-10-21

### Added

- Un archivo Markdown en la pestaña Revisión tiene un botón de vista previa junto a "Añadir archivo como contexto" que muestra el archivo renderizado, con tablas y enlaces, en lugar de su diff. Púlsalo de nuevo para volver al diff. En la pantalla Pull requests, solo los archivos que añade el pull request lo tienen.
- Ctrl+F (Cmd+F en macOS) en un archivo abierto en la pestaña Código busca texto en él. Una barra bajo el nombre del archivo resalta cada coincidencia, las recorre con Enter y Shift+Enter, y puede distinguir mayúsculas, buscar palabras completas o usar una expresión regular. Su botón de lista muestra bajo la barra cada línea con una coincidencia, y al hacer clic en una el código se desplaza hasta allí. Escape la cierra.
- El tooltip de la tarjeta de una sesión nombra el modelo que la sesión está ejecutando, en Claude Code, Codex y Kiro CLI. Cuando ya no es el modelo con el que se abrió la sesión, tras un `/model` por ejemplo, la línea dice "Abierta con …, ejecutando …".
- La pantalla Pull requests edita el título y la descripción de un pull request y los envía a GitHub. El lápiz tras el título lo renombra en el sitio: Enter o hacer clic fuera guarda, Escape cancela. Editar junto a la Descripción del Resumen la abre como Markdown, y se guarda con Guardar o Ctrl+Enter (Cmd+Enter en macOS). Una descripción larga se desplaza dentro del campo, con una barra de desplazamiento, para que Guardar siga en pantalla. Cada una envía solo lo que cambió, así que guardar la descripción nunca deshace un cambio de nombre hecho mientras tanto en GitHub.
- Un perfil puede tener su propio color e imagen. En Ajustes › Proveedores, el menú ⋯ de un perfil tiene Color, con los mismos ocho colores que una tarjeta de sesión, y Elegir imagen…, que acepta un PNG, JPEG, GIF o WebP de hasta 10 MB y conserva su cuadrado centrado. La imagen o el color se muestra dondequiera que aparezca el perfil: la lista y el selector de perfiles, el pie de la barra lateral y Continuar en. Quitar imagen vuelve a la inicial sobre el color del perfil.

### Changed

- Un bloque de tarjetas en la barra lateral, una worktree, una carpeta o el bloque fijado, ahora crece al abrirse su primera tarjeta y se encoge con la última, cuando antes aparecía y desaparecía de golpe. Los encabezados que se muestran cuando la lista se divide en bloques se despliegan igual, y se pliegan cuando queda uno solo.
- Las comprobaciones de un pull request que siguen en ejecución ahora muestran un spinner girando, en su pestaña Comprobaciones y en la línea de estado sobre las pestañas, donde antes había un reloj detenido.
- Los botones ya no se hunden un píxel mientras se pulsan, así que todos los botones de la ventana se comportan igual: los que tenían este movimiento ahora coinciden con los que nunca lo tuvieron.

### Removed

- **Breaking:** lich ya no ejecuta `.lich/setup-worktree.sh` cuando `lich open --worktree` o el `open_session` de un agente inicia una sesión en una worktree nueva. El archivo de tu repositorio no se toca. Si tus worktrees dependían de él para instalar dependencias, ejecútalo tú mismo en la terminal de la nueva worktree (`sh .lich/setup-worktree.sh`), o pídeselo al agente. La copia de tus archivos `.env*` ignorados por git (o de lo que nombre `.worktreeinclude`) y el puerto de servidor de desarrollo `LICH_WORKTREE_PORT` siguen igual.

### Fixed

- El `git commit`, `rebase` o `stash` de un agente ya no falla de vez en cuando con "Unable to create '.git/index.lock': File exists" porque lich estaba leyendo el estado o el diff de ese checkout en el mismo momento. Las lecturas de lich dejan el índice en paz.
- La tarjeta de una sesión cerrada vuelve a encogerse fuera de la barra lateral, cuando antes desaparecía de golpe.
- Ctrl+Z en una sesión ya no congela a su agente para siempre. Antes suspendía al agente, y nada en lich podía devolverlo a la vida; ahora lich mantiene la tecla fuera de la sesión.
- Las barras del tooltip de uso del plan, en el pie, vuelven a mostrar su pista vacía en los temas cuyos colores de popover y muted son iguales, como Everforest, donde solo se dibujaba la parte rellena.
- Abrir o cerrar la lista de pull requests en la pantalla Pull requests ya no hace que la pestaña del proyecto activo suelte el panel que tiene debajo y lo agarre de nuevo. La pestaña permanece conectada todo el tiempo y pasa a la lista cuando esta se desliza por debajo.

## [0.74.0] - 2026-10-20

### Added

- F11 pone la ventana en pantalla completa y la devuelve a su tamaño, sin los botones de minimizar, maximizar y cerrar mientras está así. El atajo se puede cambiar en Ajustes › Atajos.

### Changed

- Un pull request que tiene conflictos con su base ya no lista los archivos en conflicto bajo su título. El estado "Conflictos con" los cuenta, y al hacer clic en él se abre un panel sobre la pantalla que nombra cada uno.
- **Resolver conflictos** y **Corregir errores de CI** en un pull request, y una worktree iniciada desde una issue `#123`, ahora envían su mensaje a la sesión que abren, en lugar de dejarlo en el prompt esperando a que pulses Enter. A una tarjeta de terminal de ese checkout nunca se le envía nada.

### Fixed

- La pestaña del proyecto activo ahora se separa de la isla del terminal cuando abrir o redimensionar el panel Código o Revisión hace que se superponga al panel, y vuelve a conectarse cuando cabe de nuevo.
- En macOS, los botones de cerrar, minimizar y zoom quedan centrados en la barra de título de la ventana nativa, en lugar de pegados a su borde superior, y siguen el zoom de la interfaz.
- La ventana se puede arrastrar por su barra superior: pasada la última pestaña y en los huecos alrededor de la marca y de los botones. Antes, ninguna parte de la barra la movía.
- La pantalla Pull requests mantiene al día lo que muestra sin pulsar Actualizar. Vuelve a leer un pull request cada pocos segundos mientras GitHub aún calcula si se puede fusionar, una vez por minuto mientras la pantalla está abierta y cuando vuelves a la ventana, de modo que un push, una fusión o un conflicto hechos fuera de lich aparecen solos.

## [0.73.0] - 2026-10-20

> [!IMPORTANT]
> **Todos: la ventana nativa es ahora la única ventana de lich.** lich la abre en cada
> inicio; la ventana web y su Chromium integrado desaparecen.

### Added

- **Notificaciones de escritorio en las que puedes hacer clic, en la ventana nativa de Windows y macOS.** Como en Linux, una sesión que necesita tu intervención o terminó su turno muestra una notificación con Abrir y Descartar: Abrir lleva lich a la tarjeta de esa sesión, y la notificación desaparece cuando respondes o abres la sesión. En macOS requiere ejecutar lich desde Lich.app y pide permiso una sola vez; en Windows, una sesión que te espera mantiene su notificación en pantalla hasta que actúes sobre ella.
- Ajustes › Proveedores avisa bajo Perfil cuando las sesiones arrancan con un binario personalizado, con su ruta y botones para mostrarlo o volver al predeterminado. Un script que define su propio inicio de sesión o directorio de configuración se impone al perfil, y el ajuste Binario queda plegado al final de la pantalla.

### Changed

- Cuando el proceso de una sesión termina, ahora un aviso en la esquina inferior derecha pregunta qué hacer, en lugar de la banda al pie del terminal. Una tarjeta con una conversación ofrece Retomar, que vuelve a abrir esa misma conversación (en el perfil en que empezó), Empezar otra, que inicia una nueva en la tarjeta como hacía Reiniciar, y Cerrar sesión. Un shell, o un agente que terminó antes de su primer prompt, ofrece Reiniciar y Cerrar sesión. Descartar el aviso deja la tarjeta como está, y enfocar la tarjeta lo trae de vuelta.
- **El perfil del proyecto vive al pie de la barra lateral, no en cada tarjeta.** Con más de un inicio de sesión para un proveedor, la última línea de la barra lateral indica el perfil en que arrancan las nuevas sesiones de este proyecto: el proveedor de la tarjeta en pantalla, o el predeterminado para un terminal. Haz clic para elegir otro inicio de sesión, con el uso de cinco horas de cada uno, o para volver al de Global; Otros proveedores cambia el resto desde el mismo menú. Las tarjetas ya no llevan la etiqueta del perfil junto a su rama.

### Removed

- La ventana web. lich abre la ventana nativa, y una versión ya no incluye Chromium. La opción Ventana en Ajustes (web o nativa) y el elemento de la bandeja que abría la otra ventana desaparecen, igual que `lich -- <chromium flags>`, `lich --shell` y `LICH_SHELL`. Si la ventana nativa no puede arrancar, lich dice por qué y dónde está su log.

### Fixed

- Iniciar sesión en un perfil de Claude Code o Codex ahora muestra que está autenticando cuando terminas en el navegador, o cuando envías un código pegado, mientras que antes el diálogo seguía esperando el inicio de sesión hasta el final y el campo del código solo se vaciaba.
- En portugués de Brasil, todos los mensajes llaman ahora a un worktree "a worktree", donde algunos decían "o worktree".
- En macOS la ventana nativa vuelve a tener sus botones de cerrar, minimizar y zoom; desde 0.72.0 la barra superior dejaba su espacio vacío.
- En Windows, actualizar lich desde la ventana nativa ya no se revierte con "Setup was unable to automatically close all applications": la ventana seguía abierta sobre `lich-native.exe`, que el instalador debe reemplazar. Ahora se cierra antes de que corra el instalador y se abre de nuevo cuando lich regresa. Actualizar desde 0.72.0 o anterior todavía lo sufre una vez: finaliza `lich-native.exe` en el Administrador de tareas cuando aparezca el mensaje, elige Reintentar y abre lich desde el menú Inicio después.
- Cerrar la ventana nativa mientras lich no está, tras salir o mientras se reinicia, la cierra, sea cual sea el valor de "Al cerrar la ventana". Con Preguntar o Salir seguía abierta y solo mostraba un error de que no se pudieron contar las sesiones.
- En una ventana de Ajustes estrecha, un ajuste cuyo control es demasiado ancho para compartir la línea, como el selector de Perfil de un proveedor, ahora coloca el control bajo su título y descripción, donde el texto quedaba apretado en una franja y se partía letra por letra. El enlace de Documentación de un proveedor también pasa bajo su nombre en lugar de chocar con el interruptor Activado.
- Continuar en puede mover una conversación de Claude Code de vuelta al perfil que acaba de dejar, donde un hook de plugin que siguió corriendo después de terminar la sesión (el de WakaTime, por ejemplo) dejaba un archivo y el traslado se rechazaba con "already holds".

## [0.72.0] - 2026-10-19

### Changed

- La barra superior de la ventana nativa es ahora su barra de título: la barra del sistema desaparece, y minimizar, maximizar y cerrar quedan junto a Ajustes, dibujados como los dibujan Windows, GNOME y KDE y en el lado donde los pone tu escritorio; en macOS los semáforos quedan en la barra superior. Arrastra la parte vacía de la barra superior para mover la ventana y haz doble clic en ella para maximizarla. Los gestores de ventanas en mosaico conservan sus propias decoraciones.

### Fixed

- `lich quit` ya no falla con "EOF" cuando lich se cierra antes de responder: espera a que el puerto se cierre en lugar de tomar la respuesta perdida como un rechazo.
- En Wayland, la ventana nativa muestra el cursor del propio escritorio, con su tema y su tamaño, donde podía mostrar otro tema. En un monitor con una escala menor que la del más grande, el cursor ya no salta al entrar en la ventana.
- En Linux, la ventana nativa se agrupa bajo el icono propio de lich en el dock, donde aparecía como una app aparte con un icono genérico.
- En la Revisión de la ventana nativa y en los Archivos de un pull request, la barra de desplazamiento mantiene su tamaño mientras te desplazas, donde crecía y se encogía según el archivo que llenaba la vista. Arrastrarla ahora recorre el diff de forma uniforme, no archivo por archivo.

## [0.71.2] - 2026-10-19

### Fixed

- La ventana nativa vuelve a abrirse en Windows y macOS, donde se cerraba al
  instante buscando fontconfig para dibujar las vistas de código y de diff.
- Los emojis en la terminal de la ventana nativa se dibujan en color en Windows,
  donde salían solo como contornos.
- El comando de actualización que se ofrece a una instalación en macOS ahora hace tap de `lichdotdev/tap` antes de actualizar. Una instalación desde el antiguo `omartelo/tap` fallaba con "Cask 'lichdotdev/tap/lich' is unavailable", ya que Homebrew no sigue por sí solo un tap que se movió; haz el tap una vez con `brew tap lichdotdev/tap` para actualizar desde un lich anterior.
- En la ventana nativa, "Desplegar pegados largos" ahora despliega los pegados de Claude Code y Kiro CLI: el portapapeles, un lote de comentarios de revisión, el prompt con el que un pull request o issue abre una sesión y las rutas de archivo soltadas o adjuntadas.

## [0.71.1] - 2026-10-18

### Fixed

- La ventana nativa en Linux ya no se cierra con una instrucción ilegal al abrir un terminal en una CPU sin AVX-512.

## [0.71.0] - 2026-10-17

### Added

- **Diseños divididos en la ventana nativa (experimental).** Coloca lado a lado
  sesiones de cualquier proyecto abierto y divídelas como quieras:
  Ctrl+Shift+G divide la sesión enfocada hacia la derecha, Ctrl+Shift+I hacia
  abajo, y cada una pregunta qué sesión va ahí; también puedes arrastrar una
  tarjeta de la barra lateral al borde de un panel. Reorganiza un diseño
  arrastrando un panel por su cabecera hasta el borde de otro panel, o hasta su
  centro para intercambiar los dos, y alterna una pareja entre lado a lado y
  apilada con Ctrl+Shift+R. Cada diseño es una pestaña después de los
  proyectos, con el estado de sus paneles, y se renombra con doble clic;
  arrastra el espacio entre dos paneles para redimensionarlos, amplía uno con
  Ctrl+Shift+M, muévete entre ellos con Ctrl+Shift+F y saca uno con
  Ctrl+Shift+W; un diseño que se queda en un solo panel se cierra. Si dejas
  una sesión arrastrada sobre otra pestaña, esa pestaña se abre, como hace un
  navegador con un archivo arrastrado. La barra lateral y el pie de ventana
  siguen al panel enfocado.
- **Muestra al lado los delegados de una sesión.** En la ventana nativa, una
  sesión que abrió otras, en cualquier proyecto, las ofrece en el pie de
  ventana: Mostrar al lado la coloca con sus delegados apilados a su derecha,
  todos los que quepan. En un diseño, la cabecera de su panel cuenta cuántos
  hay y añade el resto, preguntando antes de sacar uno de otro diseño. Una
  tarjeta en un diseño lo indica con su nombre.
- **Las comprobaciones obligatorias están etiquetadas.** En la ventana nativa,
  la pestaña Comprobaciones de un pull request marca con Obligatorio las
  comprobaciones que condicionan su merge, como hace GitHub.

### Changed

- **lich vive en `lichdotdev/lich`.** El repositorio se mudó a la organización
  [lichdotdev](https://github.com/lichdotdev) y el sitio a
  [lichdotdev.github.io/lich](https://lichdotdev.github.io/lich/). Los enlaces
  antiguos de GitHub, los clones y las comprobaciones de actualización
  redirigen; los marcadores del sitio antiguo no.

### Fixed

- **Novedades sigue el idioma al que cambias.** En la ventana nativa, las notas
  de la versión seguían en el idioma en que se leyeron por primera vez después
  de cambiar el idioma de la interfaz, bajo nombres de grupo en el nuevo.
- **Los diffs ya no se quedan colgados con texto CJK ni acumulan memoria por
  archivo.** En la ventana nativa, una Revisión o los Archivos de un pull
  request con texto chino, japonés o coreano podían congelarse durante segundos
  al buscar cada carácter nuevo, y el diff de cada archivo guardaba su propia
  copia de las fuentes, de modo que un pull request grande ocupaba cientos de
  megabytes.

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

## [0.69.0] - 2026-10-16

### Added

- **La ventana nativa en macOS, experimental.** Lich.app la incluye junto a lich
  en Apple Silicon e Intel, y Ajustes › Apariencia › Ventana la ofrece igual
  que en Linux y Windows. Option escribe caracteres en su terminal, como en
  Terminal.app, así que los atajos Option+letra de un agente no llegan hasta
  ella. Por ahora solo se ha ejecutado en las máquinas macOS de GitHub: las
  notificaciones son simples y no se puede hacer clic en ellas, y no hay
  elemento en la barra de menús.
- **El árbol de archivos reconoce muchos más tipos de archivo.** Java, Gradle,
  C y C++, C#, PHP, Ruby, Swift, SQL, Vue, Svelte, Terraform, Dockerfile,
  .gitignore y unos treinta más ahora muestran su propio icono en el árbol de
  archivos y en los resultados de búsqueda, donde antes mostraban un glifo de
  archivo genérico.

### Fixed

- **`lich quit` cierra la ventana nativa.** Se quedaba abierta mostrando lich
  como perdido una vez que el backend había terminado; Salir de lich en la
  bandeja y una señal de parada también la cierran, mientras que un reinicio
  tras una actualización sigue conservándola.
- **La ventana nativa ofrece eliminar una worktree cuando se cierra su última sesión.** El
  aviso Se cerró gana Eliminar worktree junto a Deshacer; una worktree que lich no creó,
  o una con cambios sin confirmar, se nombra en un diálogo antes de eliminarla.
- **El terminal de la ventana nativa toma la celda bajo el puntero.** Un clic,
  una selección con arrastre, un enlace y un programa que sigue el ratón leían
  todos el puntero desplazado por el relleno del terminal, así que la celda
  que tomaban quedaba por debajo y a la derecha de la señalada, por casi una
  celda entera en una pantalla escalada.
- **El terminal de la ventana nativa dibuja emoji.** Una fuente de emoji a
  color como Noto Color Emoji guarda cada emoji como una imagen y no como un
  contorno, y el terminal solo dibujaba contornos, así que cada emoji aparecía
  como un hueco en blanco; ahora se dibujan a color, ocupando dos celdas.
- **El árbol de archivos de la ventana nativa muestra el icono de lenguaje de cada archivo.**
  Todos los archivos dibujaban el mismo glifo de archivo genérico; ahora cada
  uno lleva el icono que le da la ventana del navegador.
- **Un pull request que abre un agente aparece en su tarjeta al terminar el turno.**
  La insignia solo volvía a consultarse cuando cambiaba la rama o su último
  commit, y abrir un pull request no cambia ninguno de los dos, así que
  esperaba a que la ventana perdiera y recuperara el foco.
- **La ventana nativa se abre en Windows.** Buscaba las fuentes mediante
  fontconfig, que Windows no incluye, así que se cerraba al iniciar y lich
  recurría a la ventana web. Ahora, en Windows, encuentra por sí misma las
  fuentes instaladas, con Cascadia Mono o Consolas para el terminal por
  defecto, y los selectores de fuente de Ajustes las listan.
- **Cambiar entre un tema claro y uno oscuro ya no congela la ventana
  nativa.** A una sesión que sigue el esquema de color del terminal, como hace
  Claude Code, se le avisaba del cambio mientras la ventana todavía conservaba
  sus terminales, así que la ventana se colgaba y se cerraba.
