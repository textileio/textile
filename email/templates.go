package email

const headerMsg = `Dear developer,
`

const footerMsg = `
Use of the service and website is subject to our Terms of Use (https://docs.textile.io/policies/terms/) and Privacy Statement (https://docs.textile.io/policies/privacy/).

If you have any concerns about this email, file an issue at https://github.com/textileio/textile/issues.

Thanks for your contributions to the Textile community!`

const verificationMsg = headerMsg + `
To complete the login process, follow the link below:

{{.Link}}
` + footerMsg

const inviteMsg = headerMsg + `
{{.From}} has invited you to the {{.Org}} organization on the Hub.

To accept the invitation, follow the link below:

{{.Link}}

If you don’t want to accept it, simply ignore this email.
` + footerMsg
