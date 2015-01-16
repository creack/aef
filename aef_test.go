package aef

import "log"

func ExampleConfig() {
	// MailgunArgs represent the Mailgun configuration
	type MailgunArgs struct {
		Domain    string `json:"domain" aef:"domain" aef_usage:"mailgun domain"`
		APIKey    string `json:"api_key" aef:"api-key"`
		PublicKey string `json:"public_key" aef:"public-key"`
		TplFile   string `json:"tpl_file" aef:"tpl-file"`
	}

	mga := &MailgunArgs{}
	if err := Load(mga, "~/.mailgun.json"); err != nil {
		log.Fatal(err)
	}
}
