package config

import (
	"runtime"
	"time"
)

// PerformanceConfig contiene configuraciones de rendimiento
type PerformanceConfig struct {
	// Configuración de concurrencia
	MaxWorkers       int `json:"max_workers"`
	WorkerMultiplier int `json:"worker_multiplier"`
	BatchSize        int `json:"batch_size"`

	// Configuración de timeouts
	RequestTimeout   time.Duration `json:"request_timeout"`
	CacheTimeout     time.Duration `json:"cache_timeout"`
	ETagCheckTimeout time.Duration `json:"etag_check_timeout"`

	// Configuración de cache
	EnableCompression    bool          `json:"enable_compression"`
	MemoryCacheSize      int           `json:"memory_cache_size"`
	CacheCleanupInterval time.Duration `json:"cache_cleanup_interval"`

	// Configuración de HTTP
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
	MaxIdleConnDuration time.Duration `json:"max_idle_conn_duration"`
	ReadTimeout         time.Duration `json:"read_timeout"`
	WriteTimeout        time.Duration `json:"write_timeout"`

	// Configuración de memoria
	GCThreshold       int  `json:"gc_threshold"`
	EnableMemoryOptim bool `json:"enable_memory_optim"`

	// Configuración de pipeline
	PipelineEnabled    bool `json:"pipeline_enabled"`
	PipelineBatchSize  int  `json:"pipeline_batch_size"`
	PipelineBufferSize int  `json:"pipeline_buffer_size"`
}

// DefaultPerformanceConfig retorna la configuración por defecto optimizada
func DefaultPerformanceConfig() *PerformanceConfig {
	cpus := runtime.NumCPU()

	return &PerformanceConfig{
		// Configuración de concurrencia optimizada para I/O bound operations
		MaxWorkers:       cpus * 4,
		WorkerMultiplier: 4,
		BatchSize:        20,

		// Timeouts optimizados
		RequestTimeout:   30 * time.Second,
		CacheTimeout:     5 * time.Second,
		ETagCheckTimeout: 2 * time.Second,

		// Cache optimizado
		EnableCompression:    true,
		MemoryCacheSize:      1000,
		CacheCleanupInterval: 30 * time.Minute,

		// HTTP optimizado
		MaxConnsPerHost:     128,
		MaxIdleConnDuration: 60 * time.Second,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        10 * time.Second,

		// Memoria optimizada
		GCThreshold:       50,
		EnableMemoryOptim: true,

		// Pipeline optimizado
		PipelineEnabled:    true,
		PipelineBatchSize:  20,
		PipelineBufferSize: 100,
	}
}

// HighPerformanceConfig retorna configuración para máximo rendimiento
func HighPerformanceConfig() *PerformanceConfig {
	config := DefaultPerformanceConfig()
	cpus := runtime.NumCPU()

	// Configuración agresiva para máximo rendimiento
	config.MaxWorkers = cpus * 8
	config.WorkerMultiplier = 8
	config.BatchSize = 50
	config.MaxConnsPerHost = 256
	config.GCThreshold = 100
	config.PipelineBatchSize = 50
	config.PipelineBufferSize = 200

	return config
}

// LowResourceConfig retorna configuración para sistemas con pocos recursos
func LowResourceConfig() *PerformanceConfig {
	config := DefaultPerformanceConfig()
	cpus := runtime.NumCPU()

	// Configuración conservadora
	config.MaxWorkers = cpus
	config.WorkerMultiplier = 2
	config.BatchSize = 5
	config.MaxConnsPerHost = 32
	config.GCThreshold = 20
	config.PipelineBatchSize = 5
	config.PipelineBufferSize = 20
	config.EnableCompression = false // Menos CPU

	return config
}

// AutoOptimizeConfig ajusta la configuración basada en el sistema
func AutoOptimizeConfig() *PerformanceConfig {
	cpus := runtime.NumCPU()

	// Detectar capacidad del sistema
	if cpus >= 8 {
		return HighPerformanceConfig()
	} else if cpus >= 4 {
		return DefaultPerformanceConfig()
	} else {
		return LowResourceConfig()
	}
}

// ValidateConfig valida la configuración de rendimiento
func (c *PerformanceConfig) ValidateConfig() error {
	if c.MaxWorkers < 1 {
		c.MaxWorkers = 1
	}
	if c.MaxWorkers > 1000 {
		c.MaxWorkers = 1000
	}

	if c.BatchSize < 1 {
		c.BatchSize = 1
	}
	if c.BatchSize > 200 {
		c.BatchSize = 200
	}

	if c.RequestTimeout < time.Second {
		c.RequestTimeout = time.Second
	}
	if c.RequestTimeout > 5*time.Minute {
		c.RequestTimeout = 5 * time.Minute
	}

	if c.MaxConnsPerHost < 1 {
		c.MaxConnsPerHost = 1
	}
	if c.MaxConnsPerHost > 1000 {
		c.MaxConnsPerHost = 1000
	}

	return nil
}

// OptimizationTips retorna consejos de optimización basados en la configuración
func (c *PerformanceConfig) OptimizationTips() []string {
	var tips []string
	cpus := runtime.NumCPU()

	if c.MaxWorkers < cpus*2 {
		tips = append(tips, "Considera aumentar MaxWorkers para operaciones I/O intensivas")
	}

	if c.BatchSize < 10 {
		tips = append(tips, "Un BatchSize mayor puede mejorar el rendimiento para muchas dependencias")
	}

	if !c.EnableCompression {
		tips = append(tips, "Habilitar compresión puede ahorrar ancho de banda y espacio de cache")
	}

	if c.MaxConnsPerHost < 64 {
		tips = append(tips, "Más conexiones por host pueden mejorar el paralelismo")
	}

	if !c.PipelineEnabled {
		tips = append(tips, "El pipeline puede mejorar significativamente el rendimiento")
	}

	return tips
}

// EstimatePerformanceGain estima la mejora de rendimiento esperada
func (c *PerformanceConfig) EstimatePerformanceGain() float64 {
	cpus := runtime.NumCPU()
	baseScore := 1.0

	// Factor de workers
	workerFactor := float64(c.MaxWorkers) / float64(cpus)
	if workerFactor > 8 {
		workerFactor = 8 // Rendimientos decrecientes
	}
	baseScore *= workerFactor

	// Factor de batch
	batchFactor := 1.0 + (float64(c.BatchSize) / 100.0)
	if batchFactor > 2.0 {
		batchFactor = 2.0
	}
	baseScore *= batchFactor

	// Factor de conexiones
	connFactor := 1.0 + (float64(c.MaxConnsPerHost) / 256.0)
	if connFactor > 1.5 {
		connFactor = 1.5
	}
	baseScore *= connFactor

	// Factor de pipeline
	if c.PipelineEnabled {
		baseScore *= 1.3
	}

	// Factor de compresión (puede reducir I/O)
	if c.EnableCompression {
		baseScore *= 1.1
	}

	return baseScore
}

// ProfileType define tipos de perfiles de rendimiento
type ProfileType int

const (
	ProfileDefault ProfileType = iota
	ProfileHighPerformance
	ProfileLowResource
	ProfileAuto
)

// GetConfigForProfile retorna configuración para el perfil especificado
func GetConfigForProfile(profile ProfileType) *PerformanceConfig {
	switch profile {
	case ProfileHighPerformance:
		return HighPerformanceConfig()
	case ProfileLowResource:
		return LowResourceConfig()
	case ProfileAuto:
		return AutoOptimizeConfig()
	default:
		return DefaultPerformanceConfig()
	}
}
