package config

type MessageChannel struct {
	Position string `yaml:"position"`
	Slots    int    `yaml:"slots"`
}

type Messages struct {
	Notifications MessageChannel `yaml:"notifications"`
	Status        MessageChannel `yaml:"status"`
}

func (m Messages) Resolved() Messages {
	resolve := func(c MessageChannel, position string, slots int) MessageChannel {
		switch c.Position {
		case "topRight", "topLeft", "bottomRight", "bottomLeft":
		default:
			c.Position = position
		}
		if c.Slots <= 0 {
			c.Slots = slots
		}
		c.Slots = min(c.Slots, 8)
		return c
	}
	m.Notifications = resolve(m.Notifications, "topRight", 2)
	m.Status = resolve(m.Status, "bottomRight", 1)
	return m
}
