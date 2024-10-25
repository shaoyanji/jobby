# jobby
A resume generator composed of many different tools mostly written in go and bash and nested in a dependency package in a shell.nix file to respect your disk space.

The philosophy behind this project is that many AI models are out there which have a lot of ultra generalized knowledge. They *can* get the job but technically they are not legally yet. But if we adopt a framework where current job seekers are like a surrogate, then things get interesting.

The Problem: for an average job seeker who is trained to use MS Word to house their jpg mugshot and export to pdf is just going to bloat the hard drive relatively quickly. I have been guilty of not deleting these files because they are the unique key access data to my career arch despite them taking gigs of storage. Additionally, using these cloud services for storage in your "career" search is not very secure and not only exposes you but might signal to potential employers your lackadaisical attitude at handling secrets.

The Solution: A software that is composed of many open-source projects that employs the full potential of devops and LLMs. All the while being a little more sophisticated than copy and pasta from ChatGPT while being endlessly personalized and customizable. The current dinosaur model of a job application consists of: a CV, a cover letter, and minified media for distribution while keeping source files (your high quality assets) secure and private-ish or at your discretion. Plus, if you organize your notes and this framework correctly with Nix, it will give off the right professional vibe.

Unfortunately, yaml is written for task management but it strikes a good middle ground between readability and greppability (queryable). Lucky for you, I am writing the yaml translated into a gum UI so you dont have to remember the bash commands.

## Usage

### Installation
- clone the repo, set it to private and relax the .gitignore
- run nix-shell

### Requirements
- nix
- git

alternatively, I can provide a docker image at some point. but for now, i encourage a smarter way to deploy software

### Commands
- task generateletter
- task generatecv
- task clean


## Dependencies
- shrinkpdf.sh is a script to shrink pdf files to a given size. source: https://github.com/andreas-abel/shrinkpdf 
- tgpt
- go-task
- yq-go
- pandoc
- wkhtmltopdf
- texlive.combined.scheme-small

