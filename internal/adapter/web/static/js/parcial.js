// Intercambio de fragmentos: actualizar una fila sin recargar la pantalla.
//
// Implementa el SUBCONJUNTO de HTMX que usa este panel, con sus mismos nombres
// de atributo y su misma cabecera. Eso es deliberado: el dia que quieras HTMX
// de verdad, borras este archivo, pones el suyo, y NINGUNA plantilla cambia.
//
//   hx-post="/ruta"          a donde se envia el formulario
//   hx-target="closest tr"   que elemento se reemplaza
//   hx-swap="outerHTML"      se sustituye por lo que devuelva el servidor
//   hx-swap="delete"         se quita de la pagina
//
// Lo que NO hace, y HTMX si: hx-get, polling, historial, transiciones,
// indicadores de carga, eventos... Si algun dia hace falta algo de eso, la
// respuesta es traer HTMX, no ampliar esto.
//
// MEJORA PROGRESIVA: sin este script los formularios siguen funcionando. Son
// formularios de verdad, con action y method; el navegador los envia y el
// servidor responde con la redireccion de siempre. Este archivo solo evita el
// viaje completo. Si falla en cargar, no se rompe nada: se recarga la pagina,
// como antes.

(function () {
  "use strict";

  // resolverObjetivo entiende "closest <sel>" y cualquier selector normal.
  function resolverObjetivo(origen, expresion) {
    if (!expresion) return origen;
    var limpio = expresion.trim();
    if (limpio.indexOf("closest ") === 0) {
      return origen.closest(limpio.slice(8).trim());
    }
    return document.querySelector(limpio);
  }

  function aplicar(objetivo, modo, html) {
    if (modo === "delete") {
      objetivo.remove();
      return;
    }
    // Un <tr> suelto no se puede interpretar en cualquier contexto: el
    // analizador de HTML descarta las filas que no estan dentro de una tabla.
    //
    // createContextualFragment interpreta el HTML COMO SI estuviera en el sitio
    // donde va a ir, asi que una fila dentro de un <tbody> se conserva. Es lo
    // que hace falta aqui y lo que una plantilla suelta no garantiza.
    var rango = document.createRange();
    rango.selectNodeContents(objetivo.parentNode);
    var trozo = rango.createContextualFragment(html.trim());

    var nuevo = trozo.firstElementChild;
    if (!nuevo) {
      // Antes esto BORRABA la fila, y era peor que el problema: un fallo al
      // interpretar la respuesta se convertia en datos que desaparecen de la
      // pantalla sin que nada lo explique. Se recarga, que dice la verdad.
      window.location.reload();
      return;
    }
    objetivo.replaceWith(nuevo);
  }

  // Los contadores del titulo ("Permitidos 12") los manda el servidor en una
  // cabecera. Recalcularlos en el navegador contando filas daria un numero
  // equivocado en cuanto haya paginacion: la pagina muestra 25 de 300.
  function actualizarContadores(respuesta) {
    var total = respuesta.headers.get("X-Total");
    if (total === null) return;
    document.querySelectorAll("[data-total]").forEach(function (el) {
      el.textContent = total;
    });
  }

  function enviar(formulario) {
    var destino = formulario.getAttribute("hx-post");
    var objetivo = resolverObjetivo(formulario, formulario.getAttribute("hx-target"));
    var modo = formulario.getAttribute("hx-swap") || "outerHTML";
    if (!objetivo) return false;

    fetch(destino, {
      method: "POST",
      body: new FormData(formulario),
      // Misma cabecera que envia HTMX, para que el servidor no distinga.
      headers: { "HX-Request": "true" },
      credentials: "same-origin",
    })
      .then(function (respuesta) {
        if (!respuesta.ok) throw new Error("respuesta " + respuesta.status);
        actualizarContadores(respuesta);
        return respuesta.text();
      })
      .then(function (html) {
        aplicar(objetivo, modo, html);
      })
      .catch(function () {
        // Si algo sale mal se recarga: mas vale una pantalla lenta que una
        // pantalla que miente sobre el estado del servidor.
        window.location.reload();
      });

    return true;
  }

  // Un solo escuchador en el documento, no uno por formulario: las filas se
  // sustituyen constantemente y habria que reenganchar los escuchadores cada
  // vez. Delegar en el documento sobrevive a los reemplazos.
  document.addEventListener("submit", function (evento) {
    var formulario = evento.target;
    if (!formulario.hasAttribute || !formulario.hasAttribute("hx-post")) return;
    evento.preventDefault();
    enviar(formulario);
  });
})();

// ---------------------------------------------------------------------------
// Consola en vivo (Server-Sent Events).
//
// Se engancha al dialogo de logs: cuando la URL apunta a el, se abre el flujo;
// al cerrarlo, se corta. Mantener la conexion abierta con el dialogo cerrado
// dejaria al servidor mandando lineas que nadie mira.
//
// Sin JavaScript el enlace sigue llevando a /instances/N/logs, que es la
// pagina de siempre. Esto solo evita salir de la pantalla.
// ---------------------------------------------------------------------------

(function () {
  "use strict";

  var flujo = null;

  function cerrar() {
    if (flujo) {
      flujo.close();
      flujo = null;
    }
  }

  function pegadoAbajo(caja) {
    // Margen de 40px: si estas leyendo algo mas arriba, la consola NO debe
    // arrastrarte al final cada dos segundos. Solo sigue el directo si ya
    // estabas mirando el final.
    return caja.scrollHeight - caja.scrollTop - caja.clientHeight < 40;
  }

  function abrir(dialogo) {
    cerrar();

    var caja = dialogo.querySelector(".consola");
    var estado = dialogo.querySelector(".consola-estado");
    if (!caja) return;

    caja.textContent = "";
    if (estado) estado.textContent = "conectando…";

    flujo = new EventSource(caja.getAttribute("data-stream"));

    flujo.addEventListener("linea", function (ev) {
      var seguir = pegadoAbajo(caja);
      var linea = document.createElement("div");
      linea.className = "consola-linea";
      // textContent y no innerHTML: el log lo escribe el servidor de
      // Minecraft, e incluye nombres que eligen los jugadores. Interpretarlo
      // como HTML seria dejar que un gamertag inyecte marcado en el panel.
      linea.textContent = ev.data;
      caja.appendChild(linea);

      if (seguir) caja.scrollTop = caja.scrollHeight;
      if (estado) estado.textContent = "en vivo";
    });

    flujo.addEventListener("error", function (ev) {
      if (estado) estado.textContent = ev.data || "sin conexion; reintentando…";
    });

    // El navegador reconecta solo; esto es solo para contarlo.
    flujo.onerror = function () {
      if (estado) estado.textContent = "reconectando…";
    };
  }

  function revisarHash() {
    var dialogo = null;
    if (location.hash.indexOf("#logs-") === 0) {
      dialogo = document.querySelector(location.hash);
    }
    if (dialogo) {
      abrir(dialogo);
    } else {
      cerrar();
    }
  }

  window.addEventListener("hashchange", revisarHash);
  window.addEventListener("pagehide", cerrar);
  revisarHash();
})();
