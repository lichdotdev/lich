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
