package keymap

import "runtime"

func bind(scope Scope, action Action, what string, keys ...string) Binding {
	return Binding{Scope: scope, Action: action, Keys: keys, What: what}
}

func (b Binding) as(label string) Binding { b.Short = label; return b }

func (b Binding) hide() Binding { b.Hidden = true; return b }

// Default returns the built-in bindings in display order.
func Default() *Map {
	modeKeys := []string{"1", "2", "3", "0"}
	if runtime.GOOS == "linux" {
		modeKeys = []string{"1", "2", "3", "4", "5", "0"}
	}

	bindings := []Binding{
		bind(Global, Mode, "go to a mode", modeKeys...).as("mode"),
		bind(Global, NextMode, "next mode", "tab").hide(),
		bind(Global, PrevMode, "previous mode", "shift+tab").hide(),
		bind(Global, Help, "help", "?"),
		bind(Global, About, "about", "i"),
		bind(Global, Quit, "quit", "q"),
		bind(Global, ForceQuit, "quit at once", "ctrl+c").hide(),

		bind(Graphs, Down, "next device", "j", "down").as("device"),
		bind(Graphs, Up, "previous device", "k", "up").as("device"),

		bind(CPU, PerCore, "one plot or per core", "c").as("per core"),

		bind(Processes, Filter, "filter", "/"),
		bind(Processes, Signal, "send a signal", "K").as("signal"),
		bind(Processes, ToggleTree, "tree or flat", "t").as("tree"),
		bind(Processes, ToggleKernel, "show or hide the kernel threads", "H").as("kernel"),
		bind(Processes, ToggleLogs, "follow the selected row's journal", "L").as("log"),
		bind(Processes, Drill, "drill into the subtree", "s").as("subtree"),
		bind(Processes, Back, "back out of a drill", "esc").as("back"),
		bind(Processes, Down, "down a row", "j", "down").as("move"),
		bind(Processes, Up, "up a row", "k", "up").as("move"),
		bind(Processes, SortCPU, "sort by cpu", "c").as("sort"),
		bind(Processes, SortMem, "sort by memory", "m").as("sort"),
		bind(Processes, SortPID, "sort by pid", "p").as("sort"),
		bind(Processes, SortName, "sort by name", "n").as("sort"),
		bind(Processes, FilterContainer, "filter to the selected container", "C").as("container"),
		bind(Processes, Top, "first", "g", "home").hide(),
		bind(Processes, Bottom, "last", "G", "end").hide(),
		bind(Processes, HalfDown, "half a screen down", "ctrl+d", "pgdown").hide(),
		bind(Processes, HalfUp, "half a screen up", "ctrl+u", "pgup").hide(),

		bind(ProcessesTree, Fold, "fold the subtree", "z").as("fold"),

		bind(Containers, Down, "next container", "j", "down").as("move"),
		bind(Containers, Up, "previous container", "k", "up").as("move"),
		bind(Containers, Top, "first container", "g", "home").hide(),
		bind(Containers, Bottom, "last container", "G", "end").hide(),
		bind(Containers, EnterPane, "show its processes", "l", "right", "enter").as("processes"),
		bind(ContainerProcesses, LeavePane, "back to containers", "h", "left", "esc").as("containers"),
		bind(ContainerProcesses, Down, "down a process", "j", "down").as("move"),
		bind(ContainerProcesses, Up, "up a process", "k", "up").as("move"),
		bind(ContainerProcesses, Top, "first process", "g", "home").hide(),
		bind(ContainerProcesses, Bottom, "last process", "G", "end").hide(),
		bind(ContainerProcesses, HalfDown, "half a screen down", "ctrl+d", "pgdown").hide(),
		bind(ContainerProcesses, HalfUp, "half a screen up", "ctrl+u", "pgup").hide(),
		bind(ContainerProcesses, SortCPU, "sort by cpu", "c").as("sort"),
		bind(ContainerProcesses, SortMem, "sort by memory", "m").as("sort"),
		bind(ContainerProcesses, SortPID, "sort by pid", "p").as("sort"),
		bind(ContainerProcesses, SortName, "sort by name", "n").as("sort"),

		bind(Filtering, Apply, "apply", "enter"),
		bind(Filtering, Cancel, "cancel", "esc"),

		bind(Services, Filter, "search the list", "/"),
		bind(Services, Open, "open", "enter"),
		bind(Services, Back, "clear the search", "esc").as("clear"),
		bind(Services, Down, "down a row", "j", "down").as("move"),
		bind(Services, Up, "up a row", "k", "up").as("move"),
		bind(Services, Top, "first", "g", "home").hide(),
		bind(Services, Bottom, "last", "G", "end").hide(),

		bind(Unit, Back, "back to the list", "esc").as("back"),
		bind(Unit, Filter, "search the list", "/").hide(),
		bind(Unit, Down, "scroll down", "j", "down").as("scroll"),
		bind(Unit, Up, "scroll up", "k", "up").as("scroll"),
		bind(Unit, Top, "oldest", "g", "home").hide(),
		bind(Unit, Bottom, "newest", "G", "end").hide(),

		bind(Settings, Down, "down a row", "j", "down").as("move"),
		bind(Settings, Up, "up a row", "k", "up").as("move"),
		bind(Settings, Less, "previous value", "h", "left").as("change"),
		bind(Settings, More, "next value", "l", "right", "enter").as("change"),
		bind(Settings, Write, "write to the file", "w").as("write"),

		bind(Disk, Down, "next pane, or down a row once inside one", "j", "down").as("move"),
		bind(Disk, Up, "previous pane, or up a row", "k", "up").as("move"),
		bind(Disk, Open, "open the row", "enter").as("open"),
		bind(Disk, LeavePane, "back to the rail", "h", "left").as("pane"),
		bind(Disk, EnterPane, "into the pane", "l", "right").as("pane"),
		bind(Disk, NextPane, "next pane from anywhere", "]").hide(),
		bind(Disk, PrevPane, "previous pane from anywhere", "[").hide(),
		bind(Disk, Top, "first", "g", "home").hide(),
		bind(Disk, Bottom, "last", "G", "end").hide(),
		bind(Drives, Elevate, "read SMART as administrator", "e").as("elevate"),

		bind(Usage, Open, "into the directory", "enter").as("open"),
		bind(Usage, Back, "up a directory", "esc").as("up"),
		bind(Usage, Rescan, "look again, reusing what has not changed", "r").as("rescan"),
		bind(Usage, Remeasure, "measure everything from scratch", "R").as("remeasure"),
		bind(Usage, Delete, "delete after confirmation", "D").as("delete"),
		bind(Usage, Elevate, "retry inaccessible information as administrator", "e").as("elevate"),

		bind(Kill, Down, "next signal", "j", "down").as("move"),
		bind(Kill, Up, "previous signal", "k", "up").as("move"),
		bind(Kill, Apply, "send it", "enter").as("send"),
		bind(Kill, Cancel, "cancel", "esc", "q"),

		bind(DeleteConfirm, Apply, "delete permanently", "enter").as("delete"),
		bind(DeleteConfirm, Cancel, "cancel", "esc", "q"),
		bind(Auth, Apply, "authenticate", "enter").as("continue"),
		bind(Auth, Cancel, "cancel", "esc", "q"),

		bind(HelpScreen, Down, "scroll down", "j", "down").as("scroll"),
		bind(HelpScreen, Up, "scroll up", "k", "up").as("scroll"),
		bind(HelpScreen, Cancel, "close", "esc", "q", "?", "enter"),
		bind(AboutScreen, Down, "scroll down", "j", "down").as("scroll"),
		bind(AboutScreen, Up, "scroll up", "k", "up").as("scroll"),
		bind(AboutScreen, Cancel, "close", "esc", "q", "i", "enter"),
	}

	return newMap(bindings)
}
