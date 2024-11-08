
sops set src/letter.enc.yaml '["to"]' (yq .company tmp/template.yaml | to json)
sops set src/letter.enc.yaml '["subject"]' (yq .title tmp/template.yaml | to json)
