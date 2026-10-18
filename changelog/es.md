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
