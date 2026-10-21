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
