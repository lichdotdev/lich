## [0.71.2] - 2026-10-19

### Fixed

- La ventana nativa vuelve a abrirse en Windows y macOS, donde se cerraba al
  instante buscando fontconfig para dibujar las vistas de código y de diff.
- Los emojis en la terminal de la ventana nativa se dibujan en color en Windows,
  donde salían solo como contornos.
- El comando de actualización que se ofrece a una instalación en macOS ahora hace tap de `lichdotdev/tap` antes de actualizar. Una instalación desde el antiguo `omartelo/tap` fallaba con "Cask 'lichdotdev/tap/lich' is unavailable", ya que Homebrew no sigue por sí solo un tap que se movió; haz el tap una vez con `brew tap lichdotdev/tap` para actualizar desde un lich anterior.
- En la ventana nativa, "Desplegar pegados largos" ahora despliega los pegados de Claude Code y Kiro CLI: el portapapeles, un lote de comentarios de revisión, el prompt con el que un pull request o issue abre una sesión y las rutas de archivo soltadas o adjuntadas.
