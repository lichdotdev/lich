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
