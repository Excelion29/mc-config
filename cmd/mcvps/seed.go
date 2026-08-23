package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Excelion29/mc-config/internal/domain"
)

// runSeed prepara lo minimo para poder entrar al panel: el rol superusuario y
// su unica cuenta.
//
// No crea nada mas. Los demas roles y usuarios se gestionan desde la web, que
// es donde tiene sentido decidirlos; crearlos aqui solo obligaria a borrarlos
// despues a mano.
//
// Es idempotente y sirve tambien en produccion: las credenciales salen del
// entorno, no del codigo. Ejecutarlo dos veces no cambia nada.
func runSeed(log *slog.Logger) int {
	d, err := build(log)
	if err != nil {
		log.Error("no se pudo preparar el arranque inicial", "error", err)
		return 1
	}
	defer d.close()

	ctx := context.Background()

	if d.cfg.SuperuserEmail == "" || d.cfg.SuperuserPassword == "" {
		fmt.Fprintln(os.Stderr,
			"seed: faltan MCVPS_SUPERUSER_EMAIL y MCVPS_SUPERUSER_PASSWORD.\n"+
				"Definelos en el .env o en el entorno y vuelve a ejecutarlo.")
		return 1
	}

	existing, err := d.auth.CountSuperusers(ctx)
	if err != nil {
		log.Error("no se pudo comprobar el superusuario", "error", err)
		return 1
	}

	if err := d.auth.EnsureSuperuser(ctx, d.cfg.SuperuserEmail, d.cfg.SuperuserPassword); err != nil {
		log.Error("no se pudo crear el superusuario", "error", err)
		return 1
	}

	fmt.Println()
	if existing > 0 {
		fmt.Println("  Ya existia un superusuario: no se ha creado ninguno.")
	} else {
		fmt.Printf("  Superusuario creado: %s\n", d.cfg.SuperuserEmail)
	}
	fmt.Println("  ---------------------------------------------")
	fmt.Println("  Entra al panel con esa cuenta y crea desde ahi")
	fmt.Println("  los roles y los usuarios que necesites.")
	fmt.Println()
	fmt.Println("  Niveles sugeridos para los roles que crees:")
	for _, s := range domain.SuggestedLevels {
		fmt.Printf("    %-16s %d\n", s.Name, s.Level)
	}
	fmt.Println()
	return 0
}
