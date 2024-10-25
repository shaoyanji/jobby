default:
	gum choose $(yq . justfile) | just
generate:
	just generateletter & just generatecv

[group: 'letter']
generateletter:
	rm -rf ./temp/letter && mkdir -p ./temp/letter
	(cat ./src/letter.yaml ./src/letter.md) > ./temp/letter/letter.md
	pandoc --template=./src/letter.latex ./temp/letter/letter.md -o ./temp/letter/letter.pdf
	./src/bin/shrinkpdf.sh ./temp/letter/letter.pdf > ./output/letter.pdf
	pdfcpu optimize ./output/letter.pdf

[group: 'cv']
generatecv:
	rm -rf ./temp/cv && mkdir -p ./temp/cv
	(cat ./src/resume.yaml ./src/resume.md) > ./temp/cv/resume.md 
	pandoc ./temp/cv/resume.md -f markdown -t pdf --pdf-engine=wkhtmltopdf -c ./src/resume-stylesheet.css -s -o ./output/resume.pdf
