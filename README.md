# textile CLI

## Commands
Commands fo `textile`.

### _project_
Manages projects:

#### Subcommands
- _set-default_ <name>: sets some project as default for config
- _list_: list all projects
- <name>: prints configuration of selected project

### _init_ 
Creates a new project. returned information from backend is stored in config 
file.

Returns:
   - JWT to access API
   - Link to access the public-folder created in _storage_
   - Created wallet addr


#### Flags
- _alias_ (req): name of the project
- _filecoin_ (req): creates a FC wallet for the project
- _api_: api of the _daemon_ api used in this project.

#### Actions
- Creates a new _project_ in daemon
    - Create FC Wallet
    - Create _storage_ bucket, with a _public_ folder