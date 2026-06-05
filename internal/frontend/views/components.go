package views

func btnBase() string {
	return "cursor-pointer focus:outline-none"
}

func btnGhost() string {
	return btnBase() + " transition-colors"
}

func btnGhostIcon() string {
	return btnGhost() + " text-gray-400 hover:text-white"
}

func btnGhostSmall() string {
	return btnGhost() + " flex items-center gap-1.5 text-xs font-medium"
}

func btnOutline() string {
	return btnBase() +
		" inline-flex items-center text-center rounded-md bg-white px-3 py-2" +
		" text-sm font-semibold text-gray-900 shadow-sm ring-1 ring-inset" +
		" ring-gray-300 hover:bg-gray-50 gap-1"
}

func btnLink() string {
	return btnBase() + " text-gray-300 hover:text-white text-sm bg-transparent border-none p-0"
}
