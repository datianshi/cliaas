package commands

type SwapLoadBalancerCommand struct {
	Identifier string `long:"identifier" required:"true" description:"Identifier of the load balancer it requires to swap"`
	VmIdentifiers []string `long:"vm-identifier" required:"true" description:"Identifier of instances requires to swap"`
}

func (r *SwapLoadBalancerCommand) Execute([]string) error {
	client, err := Cliaas.Config.NewClient()
	if err != nil {
		return err
	}

	return client.SwapLb(r.Identifier, r.VmIdentifiers)
}
