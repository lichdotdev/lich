## [0.76.0] - 2026-10-21

### Added

- Un archivo Markdown abierto en la pestaña Código tiene el mismo botón de vista previa que en la pestaña Revisión, junto a la marca de solo lectura, y muestra el archivo renderizado, con tablas y enlaces, en lugar de su código fuente. Púlsalo de nuevo para volver al código fuente. Cualquier otro archivo se abre como código fuente.

### Changed

- Enviar en un lote de comentarios de revisión, en las pestañas Revisión y Código y en la pantalla Pull requests, los envía a la sesión como su prompt, cuando antes esperaban en el prompt a que pulsaras Enter. Una sesión de shell conserva los comentarios, ya que allí un Enter los ejecutaría como comando.

### Fixed

- Las teclas muertas componen en el terminal de una sesión: en macOS, ~ y luego a escribe ã, y ' y luego c escribe ç, cuando antes el acento llegaba solo antes de la letra. El texto que un método de entrada aún está componiendo, como el de un IME japonés o chino, tampoco llega ya al programa antes de confirmarse.
- Tras cambiar de modelo en una sesión de Claude Code con `/model`, el pie y el tooltip de la tarjeta de la sesión nombran el nuevo modelo en cuanto termina su primer turno. Antes conservaban el anterior hasta un turno posterior o una reconexión, porque lich leía el transcript antes de que Claude Code hubiera escrito la última línea de ese turno.
- En macOS, los botones de cerrar, minimizar y zoom se mantienen centrados en la barra de título. macOS los devolvía a la parte superior de la barra al abrirse la ventana, al cambiar a otra app y volver, y cuando el sistema alternaba entre claro y oscuro, hasta que se redimensionaba la ventana.
