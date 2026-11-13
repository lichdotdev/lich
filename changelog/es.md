## [0.90.1] - 2026-11-12

### Added

- En una extensión, una tarjeta que un nuevo renderizado coloca en otra lista de su grupo (un tablero recargado después de que otra persona la moviera) se desliza hasta allí desde donde estaba, como ya hacía dentro de una misma lista. Una tarjeta que sueltas tú, y las que aparecen o desaparecen, no se animan.

### Fixed

- En una extensión, soltar una tarjeta arrastrada en una lista ordenable ya no la pulsa: una pulsación que superó el umbral de arrastre es un arrastre, y solo un clic simple llega al manejador de la propia tarjeta.
- Una extensión con el máximo de llamadas en curso que vuelve a llamar en cuanto lee un resultado ya no es rechazada: la llamada terminada libera su lugar antes de que se envíe el resultado.

## [0.90.0] - 2026-11-12

### Added

- El navegador de una sesión abre las DevTools de Chrome para su página como una pestaña más del panel, desde un botón nuevo de la barra de herramientas: Elements, Console, Network y Sources, dibujadas en lugar de la página mientras la pestaña está seleccionada. Mientras la página está en pausa en el depurador, la pestaña muestra un signo de pausa y las herramientas de navegador del agente avisan de que la página está en pausa en vez de actuar sobre ella. El agente nunca ve ni maneja las DevTools, así que funciona igual con todos los proveedores.

### Changed

- El texto en el navegador de una sesión se ve nítido cuando la página deja de moverse: un instante después del último cambio, la página se redibuja una vez sin el desenfoque ni los halos que la imagen en movimiento tiene alrededor de las letras. Una página que muestra una foto grande conserva la imagen en movimiento, que ya se ve igual. Vale para todos los proveedores.

### Fixed

- El panel de Git se abre en cuanto se hace clic en la rama del pie. Antes esperaba a que el ratón se moviera para deslizarse hacia arriba.
- En una extensión, una lista dentro de un scroll sobre el que se arrastra un elemento se ilumina hasta el final del scroll, borde inferior incluido. Antes la iluminación se salía de la pantalla sin borde inferior.
- El panel del navegador de una sesión ya no se congela en su último fotograma en una página muy cargada a tamaño ultrapanorámico o 4K: ahora llegan a la ventana fotogramas de hasta el doble de tamaño.
- En Linux con una GPU NVIDIA, `lich-native.log` ya no dice "no GPU acceleration, the window draws on the CPU" mientras la ventana se ejecuta en la GPU. La línea venía de dibujar el icono de una notificación de escritorio, que Mesa dibuja en la CPU; ahora aparece solo cuando la propia ventana no tiene GPU.
- Al cambiar de pestaña en el navegador de una sesión ahora se ve la página que la pestaña nueva mostró por última vez, en lugar de mantener en pantalla la página de la pestaña anterior hasta que la nueva dibujaba; una pestaña que nunca estuvo delante muestra el panel vacío.
- El navegador de una sesión se mantiene nítido al mover la ventana a un monitor de otra densidad: alrededor de un segundo después del cambio, lich vuelve a iniciar el navegador a la escala nueva, con sus pestañas, la pestaña en la que estabas, sus DevTools y tus sesiones iniciadas. Lo que vive solo en la página, como un formulario a medio rellenar o cuánto habías desplazado, empieza de cero; nunca ocurre mientras la página muestra un diálogo o está en pausa en el depurador, mientras el agente actúa sobre ella, ni en un navegador abierto en su propia ventana. Cambiar el zoom de la página no lo reinicia.

## [0.89.0] - 2026-11-12

### Added

- Un recorrido guiado se abre en una instalación nueva una vez que has elegido tus agentes: te muestra cómo abrir un proyecto, iniciar una sesión, el terminal, las filas de sesión, las worktrees y dónde volver a encontrar el recorrido. Los pasos que te piden hacer algo esperan a que lo hagas, y el recorrido se aparta mientras hay un diálogo o un menú abierto. Ejecútalo de nuevo cuando quieras desde Guías, en el menú nuevo de arriba a la derecha, que también contiene Ajustes, o desde la paleta de comandos. Guías también te acompaña por las worktrees, la revisión de lo que cambió un agente, los pull requests, el traspaso del trabajo a una sesión nueva y el navegador de una sesión; una guía que necesita algo que aún no tienes, como una sesión con cambios, lo indica en el menú.
- Al hacer clic en el nombre de la rama en el pie se abre un panel de Git entre el terminal y el pie, de modo que tus sesiones siguen a la vista mientras lees el historial. Muestra todas las ramas como un grafo de colores, con ramas y etiquetas al lado y el mensaje y los archivos del commit seleccionado a la derecha; un clic en un archivo abre su diff. Busca por texto o hash, o acota por autor, fecha, ruta o rama, y una rama en la que hay una sesión lleva el anillo de esa sesión. El menú de una rama local la activa con checkout, la renombra, crea una rama nueva a partir de ella o la elimina, con Deshacer; un checkout bajo una sesión pregunta antes, y también una eliminación que dejaría commits en ninguna rama. Su altura y el ancho de sus columnas se arrastran y se conservan, se expande sobre el terminal, y Atajos de teclado en Ajustes puede asignarle una tecla.
- Con gh con sesión iniciada en más de una cuenta de GitHub, la pantalla Pull requests pregunta cuál usa un proyecto antes de listar nada, en lugar de ejecutarse sin avisar como la cuenta a la que gh cambió por última vez. Vuelve a preguntar cuando GitHub oculta el repositorio a la cuenta que usa el proyecto, así puedes elegir otra sin pasar por Ajustes.
- Las extensiones pueden tener pantallas propias. Cada pantalla que declara una extensión recibe un botón en la barra superior, entre la campana y Pull requests, y se abre sobre todo el escenario conservando la barra lateral, como hace Pull requests; los atajos de teclado propios de la extensión funcionan mientras está abierta, y puede poner el teclado en uno de sus campos, de modo que `/` puede saltar a su búsqueda, y Escape devuelve el teclado. Un punto en el botón indica que la extensión se detuvo: ámbar mientras lich la reinicia, rojo cuando se rindió, y entonces esa pantalla dice por qué, con Reiniciar y Abrir carpeta. Ajustes › Extensiones instala una desde su repositorio de GitHub, tomando la última versión compilada para tu sistema y comprobándola con los checksums de la versión, o desde una carpeta del disco, tras una confirmación que nombra la extensión, su versión y de dónde viene y enumera en palabras sencillas qué más puede hacer con tus proyectos y sesiones, sin que se ejecute nada hasta que pulses Instalar; lista todas las extensiones con su estado, versión y origen, reinicia una que se rindió, abre su carpeta y elimina una; al pasar el cursor sobre el estado de una extensión se muestra lo que puede hacer. Lo que una extensión guarda vive aparte de su carpeta y se conserva al eliminarla, así que al instalarla de nuevo, que es la forma de actualizar, retoma donde se quedó; marca Borrar también lo que guardó en la confirmación de eliminar para borrarlo también.
- Una pantalla de extensión que te deja arrastrar un elemento de una lista a otra ahora ilumina la lista en la que caería, una vacía incluida, como la barra lateral ilumina la carpeta en la que se archivaría una sesión.
- Las extensiones se pueden traducir. Una extensión que incluye traducciones muestra su nombre y los títulos de sus pantallas en el idioma de tu interfaz, en la barra superior y en Ajustes › Extensiones, y se le dice cuál es ese idioma, al arrancar y cada vez que lo cambias, para que sus propias pantallas y mensajes puedan seguirlo. Los autores de extensiones reciben el formato de traducción de lich y un paquete de Go para leerlo; consulta Translating an extension en `docs/extensions.md`.
- Las extensiones pueden leer lich y actuar sobre él: listar tus proyectos, sus worktrees y tus sesiones, abrir una sesión, darle una tarea y recoger su respuesta, hacerle una pregunta lateral a una sesión de Claude Code, enterarse cuando una sesión cambia de estado, listar, iniciar, detener y reiniciar los servicios de tus proyectos y leer lo que imprimen, leer la rama de un checkout, sus cambios, los commits por delante y por detrás de su upstream y su pull request, mostrar una notificación de escritorio, y mostrar un archivo en el dock o una página en el navegador de una sesión, como hacen tus agentes mediante las herramientas de lich. Una extensión pregunta por el mismo canal por el que dibuja y solo por lo que enumera su manifiesto, y el paquete de Go `extension` incluye un cliente que se ocupa de la contabilidad. Consulta [Escribir una extensión de lich](docs/extensions.md#calling-lich).
- Un agente puede leer por qué fallaron las comprobaciones de su pull request con `lich check-log` o la herramienta `read_check_log`, lo que hay que llamar cuando salta una vigilancia `checks-failed`: cada comprobación fallida con su enlace y, para un job de GitHub Actions, las últimas 80 líneas de cada paso que falla, sin marcas de tiempo ni secuencias de escape del terminal, unos pocos kilobytes donde `gh run view --log-failed` entrega decenas de kilobytes. Funciona en todos los proveedores, mediante la herramienta donde las herramientas de lich llegan y mediante el comando en el resto.

### Changed

- Dividir una sesión, Mostrar al lado y un navegador que se abre junto a su terminal ahora se mueven: el panel nuevo crece desde el borde por el que se dividió mientras su vecino le cede el espacio, y un panel que se saca se encoge del mismo modo, donde antes ambos saltaban. Cada terminal toma su tamaño final de una vez, así que el agente redibuja una sola vez.
- Una sesión en un diseño dividido ya no se muestra por segunda vez bajo la pestaña de su proyecto. La pestaña del proyecto muestra una sesión que no está en ningún diseño, o dice que todas las sesiones están en uno, y una sesión en un diseño lleva una marca de columnas en la barra lateral: al hacer clic en ella se va a la pestaña del diseño. Esa pestaña tiene una barra lateral propia, que lista solo las sesiones del diseño bajo el nombre de cada proyecto, con el mismo árbol de worktrees, sesiones padre e hijas y navegadores; Nueva sesión, el filtro y los servicios se quedan en la pestaña del proyecto. Sacar del diseño está en el menú de la sesión además de en el panel. Vale para todos los proveedores.
- Los anillos y los pliegues de la barra lateral se mueven como el resto de su árbol. El anillo de una sesión se redibuja con su color nuevo cuando la sesión empieza a trabajar, termina o se detiene a esperarte, donde antes cambiaba de golpe; plegar un checkout o una carpeta retira sus filas a lo largo de la línea del árbol, como ya hacía plegar una sesión sobre sus subagentes; y el anillo de una fila nueva se dibuja un poco más rápido.
- El navegador de una sesión se abre junto al terminal de la sesión, donde la sesión ya está, en lugar de en una pestaña propia que mostraba la misma sesión por segunda vez. En un diseño dividido, la página queda junto al panel de su sesión y la sigue cuando la sesión sale; cerrar la pestaña de un diseño dividido ya no deja un navegador ejecutándose fuera de la vista. El encabezado de la página tiene un botón Minimizar, que oculta la página y mantiene el navegador en marcha para el agente, junto a Cerrar, que cierra el navegador; Navegador en el menú de la sesión devuelve una página minimizada. Un navegador en marcha también aparece en la barra lateral como una fila bajo su sesión, con el nombre del sitio en el que está y atenuada mientras está minimizada: haz clic para mostrar la página, o usa su menú para minimizarla, moverla a una ventana o cerrarla. Una pestaña de navegador que dejó abierta una versión anterior no se restaura: abre el navegador de nuevo desde el menú de la sesión. Vale para todos los proveedores.
- Los avisos aparecen en la esquina inferior derecha, justo sobre el pie, y se apilan hacia arriba; antes bajaban desde arriba a la derecha, sobre los botones que una pantalla guarda ahí, como Editar y Guardar de una extensión. Las preguntas que ya esperaban en la esquina inferior derecha (una sesión cuyo proceso terminó, una conversación abierta en dos tarjetas, un navegador que abrió un agente, una oferta de perfil) ahora están en la misma pila que cualquier otro aviso.
- La interfaz suena más natural en portugués, español, japonés, coreano, ruso y chino: cada idioma se releyó entero y se reformuló donde seguía demasiado de cerca la frase en inglés, con un término por concepto en todas partes, de modo que algunas etiquetas cambiaron de nombre (la pestaña de pull request es Checks en portugués, por ejemplo). El inglés perdió sus guiones largos.
- La insignia junto al cursor del agente en el navegador de una sesión dice lo que está haciendo el agente, Haciendo clic, Escribiendo, Desplazando, Navegando y así sucesivamente, donde antes nombraba al proveedor, que la sesión ya te dice. Conserva el icono del proveedor, se queda un momento tras cada acción y luego deja la flecha sola mientras el agente piensa. Vale para todos los proveedores.
- La pestaña del navegador de una sesión decodifica cada fotograma de su página de tres a cinco veces más rápido, así que la imagen se retrasa menos respecto de la página y la ventana gasta menos CPU en una página que se anima. Compilar la ventana desde el código fuente ahora necesita `libturbojpeg` de libjpeg-turbo; `CONTRIBUTING.md` nombra el paquete.

### Fixed

- Mostrar al lado ya no responde "No hay espacio para otro panel aquí" cuando el panel de la sesión que abrió los delegados es demasiado estrecho para dividirse en dos pero lo bastante alto para apilarlos: ahora los delegados van debajo.
- Una página de ruido fino o grano de película mostrada en un panel de navegador muy grande podía enviarle a la ventana una imagen mayor de lo que acepta su conexión con el backend, lo que cortaba la ventana de todas las sesiones. Ahora esa imagen se omite, y el panel conserva la última hasta la siguiente que quepa.
- Un submenú queda junto al menú del que se abre, con un pequeño espacio, a cualquiera de los dos lados: uno que se voltea a la izquierda en el borde derecho de la ventana ya no cubre parte de su padre. El menú de la barra superior es más ancho, así que Ajustes y Guías no quedan apretados.
- El navegador de una sesión en una pantalla que no es HiDPI dibujaba su página a 1,5 veces el tamaño del panel y reducía cada fotograma para ajustarlo, lo que difuminaba el texto y mantenía una página animada en unos 30 fotogramas por segundo en un panel grande. Ahora el navegador dibuja a la escala propia de la ventana: el texto es nítido y el mismo panel logra de 49 a 60. Un navegador ya abierto conserva la escala con la que empezó, así que después de cambiar el zoom de la página o mover la ventana a un monitor de otra densidad, ciérralo y ábrelo de nuevo para tener fotogramas nítidos.
- Markdown dibuja una cita tras una barra y con texto atenuado, y un bloque de código sobre un panel propio, en la descripción y los comentarios de un pull request, las vistas previas de Markdown y las pantallas de extensiones; antes ambos se leían como el texto que los rodea.
- Un bloque de código delimitado en la descripción o en un comentario de un pull request aparecía dos veces.
- Markdown ya no dibuja un cuadro donde un bloque de código o un código en línea contiene un tabulador: un tabulador son los espacios hasta la siguiente parada de cuatro, como en la vista de código. Un bloque de código, una lista u otra cita dentro de una cita se dibuja como fuera de una, sobre su panel o tras su propia barra, donde la cita los mezclaba como texto plano, y las líneas de un bloque de código quedan tan juntas como en la vista de código. En la descripción y los comentarios de un pull request, las vistas previas de Markdown y las pantallas de extensiones.
- Una cuenta que gh lee de `GH_TOKEN` además de su llavero aparece una vez en Ajustes › Control de versiones, no dos.
- El clic de un agente en el navegador sobre un enlace que se parte en dos líneas ya no falla por estar cubierto: cae en la primera línea del enlace en lugar de en el espacio vacío entre las líneas.
- El texto chino, japonés o coreano en un terminal se dibuja con la fuente que lich descargó para la ventana cuando ninguna fuente instalada lo cubre, en lugar de como cuadros vacíos mientras el resto de la ventana se lee bien.
- Una sesión de Claude Code en un perfil ya no ve convertidos sus subagentes en tarjetas por el plugin de lich que tiene tu sesión Personal cuando la copia propia del perfil es más antigua y no puede abrirlos; decide el plugin del perfil. Una sesión de oh-my-pi en un perfil que tiene su propio `mcp.json` ahora lee los servidores de ese archivo para los nombres de herramientas de su tarjeta, no los de Personal.
- Una sesión bifurcada a una worktree con su trabajo sin confirmar trae sus propias copias de los archivos ignorados que nombra el proyecto (`.env*`, o `.worktreeinclude`), en lugar de las del checkout principal, que podían discrepar en silencio del trabajo llevado a su lado.
- Un servicio que iniciaste a mano vuelve a estar en marcha después de que lich se reinicia para una actualización, mientras su proyecto siga abierto. Cerrar lich y abrirlo más tarde sigue levantando solo los servicios guardados para iniciarse con lich.
- El navegador de una sesión abierto en su propia ventana desde el panel del navegador vuelve a seguir esa ventana: antes la página se quedaba fija en el tamaño que tenía dentro de lich e ignoraba cada cambio de tamaño de la ventana.

## [0.88.0] - 2026-11-10

### Added

- Un agente puede mostrarte el código del que habla: pídele verlo y el archivo se abre en solo lectura en la pestaña Código del dock, en las líneas que encontró, en el checkout propio de esa sesión y con tu teclado todavía en el terminal, en todos los proveedores. El agente puede dejar un comentario breve bajo esas líneas cuando tiene algo que añadir. Desde una sesión que no estás mirando, un aviso lo ofrece en lugar de tomar la pantalla.
- Ctrl+clic (Cmd+clic en macOS) sobre un `path:line` impreso en un terminal lo abre en la pestaña Código. Un nombre sin directorio, como en un stack trace de Java, se busca en el checkout, y aparece una elección cuando encaja más de un archivo.
- Las pestañas de proyecto se pueden fijar desde su menú contextual. Las pestañas fijadas quedan juntas a la izquierda de la barra y solo se cierran desde ese menú; lo fijado se conserva al reabrir la ventana.
- El menú de estado de la pantalla Pull requests tiene Aprobado: los pull requests abiertos cuya revisión está aprobada.
- Una página que un agente crea para mostrarte, un informe, un diagrama, una maqueta, se abre en el navegador de su sesión desde una carpeta propia de la sesión en lugar de caer en tu checkout, en todos los proveedores. La carpeta queda fuera de git, sobrevive a un reinicio y a una tarjeta cerrada, y desaparece cuando se elimina la sesión. Claude Code aún puede publicar una página en claude.ai con su propia herramienta Artifact; pídela en local si la quieres en la pestaña.
- El dock tiene una pestaña Artefactos: las páginas que hizo la sesión activa y, debajo, las de las otras sesiones abiertas del proyecto. Un clic abre una en el navegador de la sesión, y Adjuntar al prompt entrega una página hecha en una sesión a otra, en cualquier proveedor: se copia en la carpeta propia de la sesión y su ruta se pega en el prompt, con una línea que dice que es una copia. Un botón del pie abre la pestaña; un diseño del pie que ya guardaste la lista en Disponibles.

### Changed

- En Windows, Ctrl+V en una sesión pega de inmediato el texto del portapapeles, como hace Ctrl+Shift+V, en lugar de esperar a que el agente lea el portapapeles. Un portapapeles sin texto, una imagen o un archivo copiado, sigue llegando al agente como antes, y Alt+V sigue adjuntando una imagen cuando el portapapeles también contiene texto.

### Fixed

- Volver a abrir un archivo en la pestaña Código lo muestra ahora tal como está en disco, en lugar de la versión leída la primera vez.
- La barra lateral dibuja el encabezado de cada bloque y cuelga cada tarjeta del checkout en el que se ejecuta, como ya hacía junto a una worktree: una carpeta que contiene las únicas sesiones de un proyecto ya no desaparece, y una sesión o carpeta suelta ya no muestra tarjetas sueltas.
- Una página recargada en una pestaña del navegador después de que un agente le tomara una captura de pantalla ya no aparece comprimida en la parte superior del panel hasta que se cambia el zoom.
- Un archivo soltado o adjuntado a una sesión en sandbox antes de cerrarla sigue ahí cuando reanudas la sesión desde la pestaña Historial. Antes, la sesión reanudada no podía abrir las copias que ya se le habían entregado, y se eliminaban tres días después.

## [0.87.0] - 2026-11-08

> [!CAUTION]
> **Todos: esta versión es una actualización obligatoria.** Antes de ella, una página en cualquier navegador de tu máquina podía leer el checkout de una sesión a través del servidor de archivos locales de su navegador, el token de sesión aparecía en la línea de comandos de un proceso, y una sesión que se ejecutaba en el sandbox podía alcanzar el token propio de lich y actuar en tu máquina con él, así que actualiza ahora.

### Added

- Una versión puede exigir su actualización. Cuando una versión corrige un problema de seguridad, el aviso de actualización no tiene Más tarde y vuelve si lo cierras, y lich no abre ninguna sesión de agente nueva hasta que se instale la actualización: la ventana, `lich open` y las herramientas de un agente se niegan, cada una diciendo por qué y cómo actualizar. Las sesiones que ya están en marcha no se tocan, y aún se abre un shell para que puedas ejecutar ahí la actualización. Cuenta todas las versiones que omitiste, no solo la última, y necesita que GitHub sea alcanzable. En el diálogo de Novedades, una versión así lleva un aviso rojo.
- Una sesión puede esperar algo y luego actuar. **Programar prompt…** en su menú recibe un prompt y un momento (`45m`, `2h30`, `14:30` o una fecha completa, mostrando mientras escribes el instante en que cae) o una condición sobre el pull request de su rama: las comprobaciones fallan, las comprobaciones pasan o llega una revisión. No se gasta nada mientras nada cambia, y el prompt se escribe en la sesión como se escribe un mensaje retransmitido, en todos los proveedores. Una vigilancia de comprobaciones fallidas sigue activa con cada commit nuevo hasta que el pull request se cierra y se detiene tras 3 ejecuciones, avisándote de ello. Un reloj o un radar en la fila de la sesión indica que tiene una, y su tooltip dice cuál. Un agente puede armar lo mismo en su propia sesión con `lich schedule` o la herramienta MCP `schedule_prompt`, y lo que fijes ahí se mantiene hasta que lo cambies.
- Un agente puede ejecutar JavaScript en el navegador de su sesión y leer la respuesta (`browser_eval`, `lich browser eval`): el estado de tu app, un valor que la página no imprime, el resultado de un `fetch`. La respuesta se corta a 2000 caracteres, de modo que un estado de página grande cuesta al contexto del agente solo lo que pide. El script puede leer lo que contiene un campo, contraseñas incluidas, algo que la lectura de la página nunca le entrega.
- Un agente puede leer parte de una página en lugar de toda: `browser_info` acepta `within`, un selector CSS (`lich browser info --within <css>`), y responde con el texto y los botones y enlaces del primer elemento que coincide, de modo que la columna principal de un sitio de documentación o un diálogo abierto cuesta una fracción de la página y su navegación. Un selector que no coincide con nada, o que no es válido, responde con un error y deja funcionando los índices de la última lectura.
- La captura de pantalla del navegador de una sesión puede llevar dibujado el índice de cada botón y enlace (`annotate` en `browser_screenshot`, `lich browser screenshot --annotate`), de modo que un agente hace clic en un icono sin etiqueta por su índice en lugar de calcular dónde está. Tu panel muestra los números del momento de la captura.
- Cualquier programa puede decirle a la tarjeta de su sesión qué está haciendo. Un script de compilación, un despliegue o un agente sin hooks de lich escribe la secuencia del Program Status Protocol (OSC 7501) y la tarjeta lo muestra trabajando con su mensaje, esperándote con su pregunta o terminado. Los hooks propios de una sesión siguen ganando cuando los tiene. Los programas que preguntan si la terminal lee el protocolo reciben la respuesta. En Windows esto no se ha verificado.
- Cuando un límite de uso detiene un turno de Claude Code o Codex y otro de tus perfiles con sesión iniciada del mismo agente aún tiene cuota, un aviso ofrece continuar allí. Solo hacer clic mueve la sesión, como lo hace Continuar en, y envía la continuación de inmediato; de lo contrario la sesión espera al reinicio como antes. Un perfil cuyo plan lich no puede leer nunca se ofrece, y un clic después de que la sesión haya seguido su camino se rechaza y dice por qué.
- Un agente que entrega su trabajo puede sugerir otro de tus perfiles para el sucesor (`profile` en `handoff_session`, `lich handoff --profile`). El sucesor sigue empezando en el perfil de quien lo llama, y un aviso ofrece el sugerido cuando termina su primer turno: solo hacer clic en Continuar en lo mueve. Una sugerencia que no puede aplicarse se descarta, se le dice al agente por qué, y la entrega continúa.
- Un agente puede entregar su trabajo a una sesión nueva de sí mismo en un corte limpio, en lugar de seguir en un contexto que ha crecido demasiado. La herramienta `handoff_session` (y `lich handoff`) recibe una nota breve, con lo que está hecho, la rama y los commits, los siguientes pasos y las trampas, abre una sesión nueva en el mismo checkout con el mismo proveedor, modelo y perfil, y escribe la nota en su prompt. La sesión anterior queda abierta para que la cierres.
- Una sesión de Claude Code cuya conversación se compacta mientras tareas de otras sesiones aún esperan su respuesta recibe un recordatorio de ellas cuando la compactación termina: cada ticket, lo que preguntaba y el comando que lo responde. Antes, el resumen podía perder el ticket, y con dos tareas abiertas una respuesta que no nombraba ninguna se rechazaba, de modo que las sesiones que preguntaron esperaban por nada.
- Se detiene a dos sesiones sorprendidas enviándose mensajes de ida y vuelta sin trabajo entre medias. El segundo mensaje de ese tipo seguido llega a su agente con una nota de que el intercambio parece un bucle, y el cuarto se rechaza, de modo que la pareja deja de gastar tokens en nada. Un mensaje que sigue a trabajo real, una tercera sesión que se une o que tú escribas en cualquiera de las dos sesiones reinicia la cuenta. Lo mismo vale para un bucle a través de tres sesiones o más: un quinto mensaje retransmitido seguido, cada uno de una sesión que no hizo más que pasar el que recibió, se rechaza. Las sesiones de Crush no informan de herramientas, así que nunca se detienen.

### Changed

- Un doble clic en los widgets propios de la ventana (selección de texto, el editor, la barra de pestañas) ahora se registra con hasta 400 ms de separación en lugar de 200 ms, como ya hacían la terminal y el panel del navegador.
- En Windows, la ventana presenta los fotogramas con el modelo flip y como máximo un fotograma en cola, y un cambio de tamaño muestra su nuevo tamaño de inmediato en lugar de un fotograma anterior estirado hasta él.
- La respuesta de página del navegador de una sesión, lo que devuelve `browser_eval`, y la lista de pestañas y el identificador de página con que responden las herramientas del navegador ahora ponen lo que la página escribió (su título, texto, etiquetas de elementos, errores de consola, mensajes de diálogo) entre marcadores `<page …>` y `</page …>` con un código aleatorio que cambia en cada respuesta, y `browser_info` le dice al agente que el texto entre ellos es de la página y nunca instrucciones. Una página que escribe "SYSTEM: ignore previous instructions" ya no se lee como si hablara lich, y no puede falsificar el marcador de cierre. Añade 31 caracteres a una respuesta.
- La captura de pantalla del navegador de una sesión mide como máximo 1280 píxeles en su lado más largo, de modo que una de un panel de 2000 píxeles de ancho tiene un 60% menos de píxeles, que es lo que un agente paga en tokens cuando la mira, y un clic en un punto leído sobre ella sigue cayendo donde se leyó. Las descripciones de las herramientas del navegador, que un agente lee con cada solicitud, son una quinta parte más cortas.
- Un clic o una pulsación de tecla que deja los botones y enlaces de una página como el agente los leyó por última vez responde con una línea que lo dice en lugar de repetir toda la lista, algo que repetía aproximadamente tres respuestas de página de cada diez.
- La respuesta de página del navegador de una sesión nombra adónde va un enlace solo cuando el enlace no tiene etiqueta propia: el agente hace clic en un enlace con etiqueta por su índice, y las direcciones eran una sexta parte de lo que leía.
- Un botón o enlace en el navegador de una sesión conserva su índice mientras está en la página, de modo que el índice de un agente sigue haciendo clic en él después de que la página añada algo encima o lo desplace fuera de vista, y un índice de una página que el agente dejó falla en lugar de hacer clic en lo que tomó su número. Antes, cada lectura volvía a numerar la página desde 0.
- La respuesta de página del navegador de una sesión omite del texto de la página cada línea que es solo la etiqueta de un botón o enlace que lista justo debajo, y corta el texto en 1000 caracteres en lugar de 1200: una décima parte menor, con tanto texto propio de la página como antes.
- Un clic, una escritura y una pulsación de tecla en el navegador de una sesión sobre un elemento por índice o selector fallan antes de enviarse, nombrando lo que estorba, cuando otro elemento cubre el punto de clic del objetivo (un banner de cookies, el fondo de un modal: `covered by <div#consent-banner>`) o el objetivo está desactivado, en lugar de hacer clic en lo que haya encima o no hacer nada y responder con la página como antes. Un clic en una x y una y elegidas por el agente no cambia.

### Fixed

- El diálogo Nueva carpeta se abre con el cursor en su campo de nombre, de modo que puedes escribir el nombre de inmediato en lugar de hacer clic primero en el campo.
- El prompt que lich aparca para continuar un turno que un límite de uso detuvo vuelve a mostrarse en la sesión, y puede cancelarse desde su menú. Lo mismo ocurre con el aviso de que eliminar una sesión también eliminó un prompt programado en ella. Ambos se perdieron con la ventana web.
- `GIO_DAMAGE=on` redibuja en un framebuffer que lich conserva en lugar de fiarse de la edad del búfer del controlador, de modo que un controlador que la informa mal ya no deja la ventana con píxeles viejos.
- En X11, una muesca de la rueda se desplaza una vez en lugar de dos, y el hover se borra cuando el puntero sale de la ventana, también en Windows.
- A las sesiones de Codex y Kiro CLI ahora se les dice que se ejecutan dentro de lich y cuándo abrir otra sesión para el trabajo, en el idioma de tu prompt: ambas CLI descartan lo que el servidor MCP de lich le dice al modelo, así que tenían las herramientas y ninguna palabra sobre usarlas. Codex lo recibe en conversaciones nuevas, salvo que tu `config.toml` fije `developer_instructions`, que lich deja en paz; Kiro lo recibe cuando lich actualice su plugin allí.
- Un clic o una pulsación de tecla de un agente justo después de hacer clic en un enlace de descarga en un navegador de sesión recién iniciado ya no queda borrado por la recarga de página que hace Chrome cuando empieza esa descarga: lich retiene la segunda acción hasta que la recarga haya aterrizado, o hasta que el navegador tenga 6 segundos.
- Un worktree creado sin nombre ya no falla con "which already exists" cuando el nombre que lich eligió para él cae en la carpeta de un worktree cuya rama fue renombrada. Ahora lich omite cualquier nombre cuya carpeta esté ocupada.
- Un worktree que git crea pero no puede leer como checkout se elimina junto con el error. Antes, su carpeta quedaba atrás, y crear un worktree con ese nombre de nuevo fallaba con "which already exists".
- La navegación de un agente justo después de que una página descargara un archivo ya no falla con `net::ERR_ABORTED` en Chromium 157 y posteriores, que recarga la página casi 4 segundos después de que empiece la descarga: lich carga la página de nuevo.
- Una tarea enviada a una sesión de Claude Code o Cursor CLI en una carpeta en la que ya confía llega a su prompt incluso cuando esa sesión arrancó sin lich-plugin, lo que ocurre cuando el plugin se actualiza mientras la sesión arranca. Antes, lich retenía la tarea esperando un informe que esa sesión nunca enviaría, y tras 5 minutos respondía que la tarea nunca llegó a un prompt mientras el agente esperaba inactivo en él.
- Una sesión que ha impreso más de 2 MB conserva los modos de su terminal cuando la ventana reconstruye su terminal, al mostrarla por primera vez o tras una reconexión. Antes, el ratón podía dejar de funcionar o volver a una codificación antigua, un pegado podía enviarse en su primer salto de línea, las teclas llegaban sin el protocolo kitty, una app de pantalla completa dibujaba sobre la propia pantalla del shell, y la paleta, el primer plano, el fondo y los colores del cursor que establecía se revertían. Se aplica a todos los proveedores.
- Los emoji formados por varios caracteres se muestran como uno en la terminal, tan anchos como los dibujan otras terminales: una familia es un emoji en lugar de tres, una bandera es la bandera en lugar de dos letras, y el resto de la línea ya no se desplaza hacia la derecha. Una letra seguida de un acento combinante muestra el acento. Copiar el texto o abrir un enlace conserva todos los caracteres.

### Security

- Una sesión que se ejecuta en el sandbox ya no tiene el token propio de lich. Se le entrega uno acuñado solo para ella, que lich acepta únicamente para las llamadas que un agente confinado necesita: enviar mensajes a las sesiones a su lado, abrir y dirigir sesiones, manejar su propio navegador, leer los servicios del proyecto y su coste. Iniciar o editar un servicio (que ejecuta un comando en tu máquina, fuera del sandbox), cambiar un ajuste como el propio nivel del sandbox, escribir en la terminal de otra sesión y cualquier otra llamada siguen rechazadas, con un mensaje que dice por qué. Una sesión abierta desde dentro del sandbox también queda confinada, sea cual sea el ajuste del sandbox para ese checkout, y una sesión confinada solo puede enviar mensajes a, insertar en, controlar o interrogar sesiones que a su vez estén confinadas: un mensaje lo ejecuta el agente que lo recibe, así que uno enviado a una sesión sin confinar se ejecutaría en tu máquina. Las respuestas llegan a cualquiera, que es como un trabajador confinado informa de vuelta. Una sesión confinada tampoco puede forzar el cierre de un worktree.
- El sandbox oculta el directorio propio de lich y los worktrees de las otras sesiones. `~/.config/lich` (el token de lich, su base de datos, los perfiles de tus otras cuentas) y `~/.local/share/lich/worktrees` eran legibles dentro de una sesión confinada, como parte de `~/.config` y `~/.local/share`; ahora un directorio vacío cubre cada uno, con solo el perfil propio de la sesión, los archivos soltados, los scripts de hooks y el checkout montados de nuevo.
- El servidor de archivos locales del navegador de una sesión, el que sirve el checkout de una sesión en `http://127.0.0.1:<port>/`, ahora responde solo a las solicitudes dirigidas a `127.0.0.1:<port>` o `localhost:<port>`. Una página en tu navegador ordinario podía apuntar un nombre que controla a tu máquina y leer el checkout a través de él, archivos `.env` incluidos; una solicitud así ahora se rechaza.
- El comando que lich pega para terminar una autoactualización envía el token de sesión a lich en una cabecera de solicitud pasada por la entrada estándar de curl, de modo que ya no aparece en la línea de comandos de un proceso, que cualquier usuario de la máquina puede leer. El listener de loopback acepta `Authorization: Bearer <token>` junto a `?token=`. Lo mismo vale para el reinicio que hace `install.sh`.
- El propio directorio de configuración de lich (`~/.config/lich` en Linux) pasa a ser legible solo por tu usuario, en cada inicio, de modo que una instalación existente también se endurece. Los prompts y el texto de las sesiones viven en la base de datos de ahí. El directorio superior se deja como estaba.

## [0.86.0] - 2026-11-07

### Added

- La bandeja te dice qué sesiones te necesitan. Su menú lista las sesiones que esperan algo de ti, con lo que preguntan, y aquellas cuyo turno terminado aún no has leído, y después las que trabajan, con la herramienta que ejecutan; al hacer clic en una, la ventana se abre en su tarjeta. Pasar el puntero por el icono las lista todas, sin recortar, donde tu bandeja muestra el tooltip del icono, y en macOS el tooltip de cada entrada trae su línea completa. El icono lleva un punto cuando hay algo en marcha: ámbar para una sesión que espera algo de ti, verde para un turno sin leer, gris mientras una trabaja. Los servicios pasaron a un submenú.
- La bandeja de notas de la página del navegador de una sesión tiene un cuadro para una nota sobre todas ellas, como el lote de comentarios de revisión. Lo que escribes ahí cierra el mensaje que recibe el agente. Enter envía las notas, Shift+Enter empieza una línea nueva y Borrar la vacía junto con el resto.

### Changed

- La ventana gasta aproximadamente una décima parte menos de CPU en cada fotograma que dibuja: ya no calcula qué parte del fotograma cambió cuando de todos modos está dibujando el fotograma entero.
- Mover el puntero sobre la ventana ya no la redibuja en cada movimiento: dibuja solo cuando algo bajo el puntero cambia, y un agente que sigue el ratón (Claude Code lo hace) ya no hace que su salida se dibuje al instante mientras mueves el puntero. Menos energía gastada mientras tienes la mano en el ratón.
- Un enlace en una terminal se subraya, y el puntero se convierte en una mano, solo mientras se mantiene pulsado Ctrl (Cmd en macOS), la tecla con la que un clic lo abre, como en Ghostty. Antes, se subrayaba todo enlace bajo el puntero.
- La nota bajo un lote de comentarios de revisión, en Código, Revisión y Pull requests, ahora es un cuadro que crece a medida que escribes: Shift+Enter empieza una línea nueva y Enter la sigue añadiendo al lote.

### Fixed

- Escribir o pegar en una terminal desplazada hacia arriba la devuelve al final, donde aparece lo que escribes, como hace la mayoría de las terminales. Antes, el texto llegaba a la sesión mientras la vista seguía en el historial, y había que desplazarse hacia abajo a mano para verlo.
- El texto de la terminal tiene el mismo grosor que el resto del texto de la ventana. Antes, los bordes suaves de cada letra se mezclaban de otra manera, así que el texto claro sobre un tema oscuro se veía más fino de lo debido y el texto oscuro sobre un tema claro se veía más grueso.
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
