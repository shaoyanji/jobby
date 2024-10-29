# jobby
A composable framework for job applications.

## Motivation

The philosophy behind this project is that you can control and customize your job application using the latest disruptive tools of AI without being a surrogate.

The Problem: for an average job seeker who is trained to use MS Word to house their jpg mugshot and export to pdf is just going to bloat hard drive relatively quickly. Furthermore, discarding everything just to send, send, send doesn't provide you a history of a progressive career arc. While it is trivial to add a sqlite database and call it a day, you need a sensible way to access the data without querying while safely and securely managing private and sensitive information with a secrets management.

The Solution: use a yaml file to define your job application. yaml is used a lot in devops and it's a stringifiable query language while being human readable.

Unlike other tools that are more effective at probably getting you a job (Auto_Jobs_Applier_AIHawk)[https://github.com/feder-cr/Auto_Jobs_Applier_AIHawk]. I am just simply practicing my yaml so I don't have to remember bash commands. While at the same time searching for a job and improving my CV using tools that a dependencies-managed in the framework of CLI compos-ability.

## Usage

### Requirements
- nix
- git

### Installation
- clone the repo, set it to private and the .gitignore to your preferences
- run `nix-shell` to enter a nix shell
- run 'task' for a list of menu options

### Commands
- task generateletter
- task generatecv
- task clean
- exit

## Dependencies
- shrinkpdf.sh is a script to shrink pdf files to a given size. source: https://github.com/andreas-abel/shrinkpdf 
- tgpt
- go-task
- yq-go
- pandoc
- wkhtmltopdf
- texlive.combined.scheme-small
- sops (for secrets management)

