package ipc

// TODO: Look into adding support for sway and hyprland ipc so that w2g can interact with those in tool mode

type (
  // A request to list the available Outputs
	OutputRequest struct {
    // Whether to include the modes an output supports
		IncludeModes    bool   `json:"include_modes"`
    // Target one specific output
		SpecifiesOutput bool   `json:"specifies_output"`
    // Name of the output you want info on. Only matters if SpecifiesOutput is set
		TargetOutput    string `json:"target_output"`
	}

  // A mode an output supports
  OutputMode struct {
    // Mode height in pixel
    Height int
    // Mode width in pixel
    Width int
    // Refresh rate of the mode in millihertz
    RefreshRate int
  }

  // Response to a OutputRequest message
  OutputResponse struct {
    // List of all outputs. Only contains target output if specified
    Outputs []string
    // A list of modes an output supports. Only set if IncludeModes is true
    OutputModes map[string][]OutputMode
    // Nr of outputs found
    OutputsFound int
  }
)
