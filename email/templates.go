package email

const headerMsg = `Dear Textile developer,
`

const footerMsg = `
Use of the service and website is subject to our <a href="https://docs.textile.io/policies/terms/">Terms of Use</a> and <a href="https://docs.textile.io/policies/privacy/">Privacy Statement</a>.

If you have any concerns about this email, file an issue at https://github.com/textileio/textile/issues.

Thanks for your contributions to the Textile community!`

const verificationMsg = headerMsg + `
To complete the login process, follow the link below:

{{.Link}}
` + footerMsg

const inviteMsg = headerMsg + `
{{.From}} has invited you to the {{.Org}} organization on Textile.

To accept the invitation, follow the link below:

{{.Link}}

If you don’t want to accept it, simply ignore this email.
` + footerMsg
