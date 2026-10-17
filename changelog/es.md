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
