package domain

// Ajustes de un mundo.
//
// Se separan en dos grupos porque se comportan distinto, y confundirlos lleva a
// una interfaz que miente:
//
//   - Los de GENERACION dan forma al terreno la primera vez y ya no se pueden
//     cambiar. Tocar la semilla de un mundo ya generado no reescribe nada: el
//     terreno esta en disco.
//   - Los de REGLAS se releen en cada arranque, asi que si se pueden cambiar.
//
// Sobre las reglas hay una decision que conviene tener presente: el panel
// reescribe la configuracion CADA VEZ que arranca una instancia, igual que hace
// con la lista de permitidos. Un administrador puede cambiar la dificultad
// dentro del juego y funcionara en esa sesion, pero al siguiente arranque
// vuelve a mandar el panel.
//
// Es a proposito. La alternativa -escribir solo al crear y no tocar mas- haria
// que la pantalla ensenara valores que ya no son ciertos, y una interfaz que
// miente es peor que una que manda.

// Gamemode es el modo de juego con el que entra la gente.
type Gamemode string

const (
	GamemodeSurvival  Gamemode = "survival"
	GamemodeCreative  Gamemode = "creative"
	GamemodeAdventure Gamemode = "adventure"
)

func (g Gamemode) Valid() bool {
	switch g {
	case GamemodeSurvival, GamemodeCreative, GamemodeAdventure:
		return true
	}
	return false
}

func (g Gamemode) Label() string {
	switch g {
	case GamemodeSurvival:
		return "Supervivencia"
	case GamemodeCreative:
		return "Creativo"
	case GamemodeAdventure:
		return "Aventura"
	}
	return string(g)
}

// Difficulty es la dificultad del mundo.
type Difficulty string

const (
	DifficultyPeaceful Difficulty = "peaceful"
	DifficultyEasy     Difficulty = "easy"
	DifficultyNormal   Difficulty = "normal"
	DifficultyHard     Difficulty = "hard"
)

func (d Difficulty) Valid() bool {
	switch d {
	case DifficultyPeaceful, DifficultyEasy, DifficultyNormal, DifficultyHard:
		return true
	}
	return false
}

func (d Difficulty) Label() string {
	switch d {
	case DifficultyPeaceful:
		return "Pacifico"
	case DifficultyEasy:
		return "Facil"
	case DifficultyNormal:
		return "Normal"
	case DifficultyHard:
		return "Dificil"
	}
	return string(d)
}

// Generation son los ajustes que solo importan al nacer el mundo.
//
// Una vez generado el terreno, cambiarlos no hace nada: por eso la interfaz
// solo los ofrece al crear, y despues los ensena como dato de consulta.
type Generation struct {
	// Seed va como texto porque Minecraft acepta palabras: "hola" es una
	// semilla valida y el juego la convierte. Guardarla como numero perderia
	// justo las que la gente escribe a mano.
	//
	// Vacia significa "al azar", que es lo que hace el juego por defecto.
	Seed string
	// LevelType es el tipo de terreno. Los valores NO coinciden entre
	// ediciones, asi que los traduce cada adaptador y aqui se guarda el
	// concepto, no la cadena que espera el servidor.
	LevelType LevelType
	// Structures: pueblos, templos, fortalezas. Apagarlo da un mundo virgen.
	Structures bool
	// BonusChest deja un cofre con herramientas junto al punto de aparicion.
	BonusChest bool
}

// LevelType es la forma del terreno.
//
// Las dos ediciones NO ofrecen los mismos: Java tiene biomas grandes y
// amplificado, que en Bedrock no existen; Bedrock tiene el heredado, que son
// los mundos pequenos de las versiones antiguas. Ofrecer uno que la edicion no
// entiende seria peor que no ofrecerlo: el servidor lo ignora en silencio y
// genera un mundo normal, y quien lo pidio no se entera.
type LevelType string

const (
	LevelNormal LevelType = "normal"
	LevelFlat   LevelType = "flat"
	// Solo Java.
	LevelLargeBiomes LevelType = "large_biomes"
	LevelAmplified   LevelType = "amplified"
	// Solo Bedrock: mundos de tamano limitado, como en las versiones viejas.
	LevelLegacy LevelType = "legacy"
)

func (l LevelType) Label() string {
	switch l {
	case LevelNormal:
		return "Normal"
	case LevelFlat:
		return "Plano"
	case LevelLargeBiomes:
		return "Biomas grandes"
	case LevelAmplified:
		return "Amplificado"
	case LevelLegacy:
		return "Heredado (mundo limitado)"
	}
	return string(l)
}

// Note explica para que sirve cada uno, porque los nombres no se explican
// solos si no has creado muchos mundos.
func (l LevelType) Note() string {
	switch l {
	case LevelNormal:
		return "el de siempre"
	case LevelFlat:
		return "sin relieve, util para construir"
	case LevelLargeBiomes:
		return "los mismos biomas, mucho mas extensos"
	case LevelAmplified:
		return "montañas enormes; exige mas al servidor"
	case LevelLegacy:
		return "mundo pequeño y cerrado, como en las versiones antiguas"
	}
	return ""
}

// LevelTypesFor da los tipos que entiende cada edicion, en el orden en que
// conviene ensenarlos.
func LevelTypesFor(e Edition) []LevelType {
	switch e {
	case EditionJava:
		return []LevelType{LevelNormal, LevelFlat, LevelLargeBiomes, LevelAmplified}
	case EditionBedrock:
		return []LevelType{LevelNormal, LevelFlat, LevelLegacy}
	}
	return nil
}

// ValidFor comprueba que el tipo exista EN ESA EDICION, no en general.
func (l LevelType) ValidFor(e Edition) bool {
	for _, v := range LevelTypesFor(e) {
		if v == l {
			return true
		}
	}
	return false
}

// Rules son los ajustes que se releen en cada arranque y por tanto se pueden
// cambiar cuando quieras.
type Rules struct {
	Gamemode   Gamemode
	Difficulty Difficulty
	// AllowCommands permite usar comandos dentro del juego. Las dos ediciones
	// lo resuelven distinto -Bedrock tiene un interruptor, Java lo ata a ser
	// operador- y de eso se encarga cada adaptador.
	AllowCommands bool
	// PvP permite que los jugadores se hagan dano entre ellos.
	PvP bool
	// MaxPlayers no es solo un limite: tambien es lo que el panel ensena como
	// "2 / 12 jugando".
	MaxPlayers int
}

// DefaultGeneration son los valores con los que Minecraft crea un mundo si no
// tocas nada. Se copian a proposito: quien crea un mundo desde el panel deberia
// obtener lo mismo que obtendria desde el juego.
func DefaultGeneration() Generation {
	return Generation{
		Seed:       "",
		LevelType:  LevelNormal,
		Structures: true,
		BonusChest: false,
	}
}

// DefaultRules son los valores por defecto de un servidor nuevo.
//
// AllowCommands va en false igual que en el juego: un mundo donde cualquiera
// puede darse objetos deja de tener gracia, y activarlo debe ser una decision.
func DefaultRules() Rules {
	return Rules{
		Gamemode:      GamemodeSurvival,
		Difficulty:    DifficultyNormal,
		AllowCommands: false,
		PvP:           true,
		MaxPlayers:    12,
	}
}
