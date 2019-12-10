# CLI
This is a high-level specification of the `textile filecoin` subcommand.

## _deal-put_ 
This is an interactive command in which deals with be presented, and later 
selected to store the specified file in Filecoin.


### Flags
- _file_ (req): file path of file to be saved
- _duration_ (req): aprox. duration in minutes to store the file
- _sort_: sort criteria to present results
- _automate_: disables interactive command and automate decisions
- _min-proposals_: minimum amount of proposals to allow decision making
- _rain_: ToDo
- _miners_: ToDo

## Actions
(High level overview, will refine when impl attempt)
- generates CID and necessary info of the file
- calls API with file info and flags provided
- Use Reputation Module to select peers to be returned.
- Present results
- Let user select miner options with `accept <idx>` interactive command
- call API with file info, and selected miner offers to execute deals
- return and present results


## _deal-get_ 
Retrieves a file by it's CID.

ToDo: Still a lot to discover about retrieving files. May be interactive as 
_deal-put_ since multiple storage miners or retrieval miners might be available 
at different costs.

### Flags
- _cid_ (req): cid of the file to be fetched
- _output_ (req): output path to save retrieved file
- ToDo: possible much more flags if this commands turns to be interactive too



## _repair_ 
Instructs the daemon to repair slashed miners which didn't satisfy deals. Do 
wahtever necessary to hold original deal intention of the file.

### Flags
- _dealid_ (req): deal id to watch
- _duration_ (req): duration in minutes to watch


# Daemon API
ToDo: High level description
ToDo: Security
ToDo: APIs section
