package dockerx

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// Estas pruebas hablan con un Docker real. Si no hay ninguno, se saltan: no
// deben romper una compilacion en una maquina sin Docker.
//
// Se usa alpine y no la imagen de Bedrock porque descargar esta ultima tarda
// minutos, y lo que se comprueba aqui es el cliente HTTP, no Minecraft.
func runtimeOrSkip(t *testing.T) *Runtime {
	t.Helper()

	rt, err := NewRuntime(os.Getenv("DOCKER_HOST"))
	if err != nil {
		t.Skipf("sin cliente de Docker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Ping(ctx); err != nil {
		t.Skipf("Docker no responde: %v", err)
	}
	return rt
}

func TestCicloDeVidaDelContenedor(t *testing.T) {
	rt := runtimeOrSkip(t)
	ctx := context.Background()

	spec := app.ContainerSpec{
		Name:     "mcvps-test-ciclo",
		Image:    "alpine:3.20",
		Cmd:      []string{"sh", "-c", "echo arrancado; sleep 300"},
		DataDir:  t.TempDir(),
		PortHost: 0,
		PortIn:   19132,
		Protocol: "udp",
		MemoryMB: 64,
		CPUs:     0.5,
	}

	// Puerto 0 deja que Docker elija uno libre: la prueba no debe chocar con
	// nada que el usuario tenga escuchando.
	spec.PortHost = 0

	id, err := rt.Create(ctx, spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { rt.Remove(context.Background(), id) })

	if err := rt.Start(ctx, id); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st, err := rt.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Exists || !st.Running {
		t.Fatalf("Status = %+v, se esperaba existente y en marcha", st)
	}

	// Los logs llegan multiplexados: si demux falla, aqui sale basura binaria.
	deadline := time.Now().Add(10 * time.Second)
	var logs string
	for time.Now().Before(deadline) {
		logs, err = rt.Logs(ctx, id, 20)
		if err != nil {
			t.Fatalf("Logs: %v", err)
		}
		if strings.Contains(logs, "arrancado") {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !strings.Contains(logs, "arrancado") {
		t.Errorf("los logs no traen la salida esperada: %q", logs)
	}

	out, err := rt.Exec(ctx, id, []string{"echo", "hola-exec"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(out, "hola-exec") {
		t.Errorf("Exec devolvio %q", out)
	}

	// Lo importante de F3: StopAndWait debe volver cuando el contenedor esta
	// realmente detenido, no cuando Docker acepta la orden (H-F0-6).
	inicio := time.Now()
	if err := rt.StopAndWait(ctx, id, 10*time.Second); err != nil {
		t.Fatalf("StopAndWait: %v", err)
	}
	transcurrido := time.Since(inicio)

	st, err = rt.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status tras parar: %v", err)
	}
	if st.Running {
		t.Errorf("StopAndWait volvio en %v pero el contenedor sigue en marcha", transcurrido)
	}
	t.Logf("apagado limpio en %v, estado final %q", transcurrido, st.Status)

	if err := rt.Remove(ctx, id); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	st, err = rt.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status tras borrar: %v", err)
	}
	if st.Exists {
		t.Error("el contenedor sigue existiendo despues de Remove")
	}
}

func TestOperacionesSobreLoQueNoExiste(t *testing.T) {
	rt := runtimeOrSkip(t)
	ctx := context.Background()

	// Borrar, consultar o parar algo inexistente no es un error para nosotros:
	// el estado deseado ya se cumple.
	if err := rt.Remove(ctx, "mcvps-no-existe-jamas"); err != nil {
		t.Errorf("Remove de algo inexistente deberia ser inofensivo: %v", err)
	}
	if err := rt.StopAndWait(ctx, "mcvps-no-existe-jamas", time.Second); err != nil {
		t.Errorf("StopAndWait de algo inexistente deberia ser inofensivo: %v", err)
	}

	st, err := rt.Status(ctx, "mcvps-no-existe-jamas")
	if err != nil {
		t.Errorf("Status de algo inexistente deberia ser inofensivo: %v", err)
	}
	if st.Exists {
		t.Error("Status dice que existe algo que no existe")
	}
}

func TestCreateReemplazaElAnterior(t *testing.T) {
	rt := runtimeOrSkip(t)
	ctx := context.Background()

	spec := app.ContainerSpec{
		Name:     "mcvps-test-reemplazo",
		Image:    "alpine:3.20",
		Cmd:      []string{"sleep", "300"},
		DataDir:  t.TempDir(),
		PortIn:   19132,
		Protocol: "udp",
		MemoryMB: 64,
		CPUs:     0.5,
	}

	first, err := rt.Create(ctx, spec)
	if err != nil {
		t.Fatalf("primer Create: %v", err)
	}
	t.Cleanup(func() { rt.Remove(context.Background(), first) })

	// Docker no admite dos contenedores con el mismo nombre. Recrear una
	// instancia tiene que funcionar sin que nadie limpie a mano.
	second, err := rt.Create(ctx, spec)
	if err != nil {
		t.Fatalf("segundo Create: %v", err)
	}
	t.Cleanup(func() { rt.Remove(context.Background(), second) })

	if first == second {
		t.Error("el segundo Create devolvio el mismo id: no llego a recrearse")
	}

	st, _ := rt.Status(ctx, first)
	if st.Exists {
		t.Error("el contenedor anterior no se retiro")
	}
}
