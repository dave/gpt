# gpt

Manipulates the route files for the Greater Patagonia Trail.

## Install

### Mac / Linux

Use [homebrew](https://brew.sh/) to install:

```
brew install dave/gpt/gpt
```

to upgrade: 

```
brew upgrade dave/gpt/gpt
```

### Windows

Download the binary from the [releases page](https://github.com/dave/gpt/releases).


## Usage

Create an empty directory, and copy `GPT Master.kmz` into it. This file is included in [the track files zip](https://www.wikiexplora.com/Greater_Patagonian_Trail#The_GPT_Track_Files).

Run the `gpt` command in that directory. 

The command will create a new directory `output` with the output files.
 

```
Usage of gpt:
  -ele
    	lookup elevations (default true)
  -output string
    	output dir (default "./output")
  -points string
    	all points file (default "./All Points.kmz")
  -stamp string
    	date stamp for output files (default "20200403")
  -tracks string
    	all tracks file (default "./All Tracks.kmz")
  -version
    	show version
```
