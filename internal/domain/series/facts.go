package series

// Text facts share the metric-series key namespace.
const (
	FactCPUModel          = "cpu.model"
	FactCPUVendor         = "cpu.vendor"
	FactCPUSockets        = "cpu.sockets"
	FactCPUCores          = "cpu.cores"
	FactCPULogical        = "cpu.logical"
	FactCPUBaseSpeed      = "cpu.base_speed"
	FactCPUVirtualisation = "cpu.virtualisation"
	FactCPUHypervisor     = "cpu.hypervisor"
	FactCPUL1d            = "cpu.cache.l1d"
	FactCPUL1i            = "cpu.cache.l1i"
	FactCPUL2             = "cpu.cache.l2"
	FactCPUL3             = "cpu.cache.l3"

	FactMemorySlots       = "mem.slots"
	FactMemorySlotsUsed   = "mem.slots.used"
	FactMemoryForm        = "mem.form_factor"
	FactMemoryType        = "mem.type"
	FactMemorySpeed       = "mem.speed"
	FactMemoryModules     = "mem.modules"
	FactMemoryMaxCapacity = "mem.max_capacity"
	FactMemoryPart        = "mem.part_number"

	// FactUnitsState uses systemd manager states such as running and degraded.
	FactUnitsState = "units.state"
	FactUnitsBoot  = "units.boot"

	FactBatteryStatus = "battery.status"
	FactBatteryHealth = "battery.health"
	FactACOnline      = "power.ac"

	FactGPUName = "gpu.name"
)
