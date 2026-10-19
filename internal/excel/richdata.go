package excel

import "fmt"

func (w *Workbook) richImages() (map[int]string, error) {
	out := map[int]string{}
	if w.files["xl/metadata.xml"] == nil {
		return out, nil
	}

	var meta struct {
		Types []struct {
			Name string `xml:"name,attr"`
		} `xml:"metadataTypes>metadataType"`
		Futures []struct {
			Name   string `xml:"name,attr"`
			Blocks []struct {
				Refs []struct {
					Index int `xml:"i,attr"`
				} `xml:"extLst>ext>rvb"`
			} `xml:"bk"`
		} `xml:"futureMetadata"`
		Values []struct {
			Refs []struct {
				Type  int `xml:"t,attr"`
				Value int `xml:"v,attr"`
			} `xml:"rc"`
		} `xml:"valueMetadata>bk"`
	}
	if err := w.parse("xl/metadata.xml", &meta); err != nil {
		return nil, err
	}

	var rv struct {
		Items []struct {
			Structure int   `xml:"s,attr"`
			Values    []int `xml:"v"`
		} `xml:"rv"`
	}
	var structure struct {
		Items []struct {
			Type string `xml:"t,attr"`
			Keys []struct {
				Name string `xml:"n,attr"`
			} `xml:"k"`
		} `xml:"s"`
	}
	var rel struct {
		Items []struct {
			ID string `xml:"id,attr"`
		} `xml:"rel"`
	}

	for _, v := range []struct {
		part string
		dest any
	}{
		{"xl/richData/rdrichvalue.xml", &rv},
		{"xl/richData/rdrichvaluestructure.xml", &structure},
		{"xl/richData/richValueRel.xml", &rel},
	} {
		if err := w.parse(v.part, v.dest); err != nil {
			return nil, err
		}
	}

	rels, err := w.rels("xl/richData/richValueRel.xml")
	if err != nil {
		return nil, err
	}

	invalid := func() (map[int]string, error) {
		return nil, fmt.Errorf("Broken richData image relationship")
	}
	for i, value := range meta.Values {
		if len(value.Refs) != 1 {
			continue
		}

		rc := value.Refs[0]
		if rc.Type < 1 || rc.Type > len(meta.Types) {
			return invalid()
		}

		name := meta.Types[rc.Type-1].Name
		if name != "XLRICHVALUE" {
			continue
		}

		index := -1
		for _, future := range meta.Futures {
			if future.Name == name && rc.Value >= 0 && rc.Value < len(future.Blocks) && len(future.Blocks[rc.Value].Refs) > 0 {
				index = future.Blocks[rc.Value].Refs[0].Index
			}
		}

		if index < 0 || index >= len(rv.Items) {
			return invalid()
		}

		record := rv.Items[index]
		if record.Structure < 0 || record.Structure >= len(structure.Items) {
			return invalid()
		}

		shape := structure.Items[record.Structure]
		if shape.Type != "_localImage" {
			continue
		}

		for k, key := range shape.Keys {
			if key.Name != "_rvRel:LocalImageIdentifier" {
				continue
			}

			if k >= len(record.Values) {
				return invalid()
			}

			idx := record.Values[k]
			if idx < 0 || idx >= len(rel.Items) {
				return invalid()
			}

			part, ok := rels[rel.Items[idx].ID]
			if !ok || w.files[part] == nil {
				return invalid()
			}

			out[i+1] = part
		}
	}

	return out, nil
}
