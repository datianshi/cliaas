package commands

type ReplaceVMCommand struct {
	Identifier string `long:"identifier" required:"true" description:"Identifier of the VM that is being replaced"`
}

func (r *ReplaceVMCommand) Execute([]string) error {
	client, err := Cliaas.Config.NewClient()
	if err != nil {
		return err
	}

	return client.Replace(r.Identifier, Cliaas.Config.Image())
}
