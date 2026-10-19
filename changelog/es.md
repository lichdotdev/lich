## [0.72.0] - 2026-10-19

### Changed

- La barra superior de la ventana nativa es ahora su barra de título: la barra del sistema desaparece, y minimizar, maximizar y cerrar quedan junto a Ajustes, dibujados como los dibujan Windows, GNOME y KDE y en el lado donde los pone tu escritorio; en macOS los semáforos quedan en la barra superior. Arrastra la parte vacía de la barra superior para mover la ventana y haz doble clic en ella para maximizarla. Los gestores de ventanas en mosaico conservan sus propias decoraciones.

### Fixed

- `lich quit` ya no falla con "EOF" cuando lich se cierra antes de responder: espera a que el puerto se cierre en lugar de tomar la respuesta perdida como un rechazo.
- En Wayland, la ventana nativa muestra el cursor del propio escritorio, con su tema y su tamaño, donde podía mostrar otro tema. En un monitor con una escala menor que la del más grande, el cursor ya no salta al entrar en la ventana.
- En Linux, la ventana nativa se agrupa bajo el icono propio de lich en el dock, donde aparecía como una app aparte con un icono genérico.
