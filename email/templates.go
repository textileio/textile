package email

const headerMsg = `Dear Textile developer,
`

const footerMsg = `
If you have any concerns about this email, file an issue at https://github.com/textileio/textile/issues.

Thanks for your contributions to the Textile community!`

const verificationMsg = headerMsg + `
To complete the login process, follow the link below:

{{.Link}}
` + footerMsg

const inviteMsg = headerMsg + `
{{.From}} has invited you to the {{.Team}} team on Textile.

To accept the invitation, follow the link below:

{{.Link}}

If you don’t want to accept it, simply ignore this email.
` + footerMsg
