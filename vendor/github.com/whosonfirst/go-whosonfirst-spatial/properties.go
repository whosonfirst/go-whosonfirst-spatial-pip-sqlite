package spatial

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/whosonfirst/go-reader"
	"github.com/whosonfirst/go-whosonfirst-spr/v2"
	"github.com/whosonfirst/go-whosonfirst-uri"
	"io"
	_ "log"
	"strings"
)

type AppendPropertiesOptions struct {
	Reader       reader.Reader
	SourcePrefix string
	TargetPrefix string
	Keys         []string
}

func PropertiesResponseResultsWithStandardPlacesResults(ctx context.Context, opts *AppendPropertiesOptions, results spr.StandardPlacesResults) (*PropertiesResponseResults, error) {

	previous_results := results.Results()

	new_results := make([]*PropertiesResponse, len(previous_results))

	for idx, r := range previous_results {

		spr_id := r.Id()

		id, uri_args, err := uri.ParseURI(spr_id)

		if err != nil {
			return nil, err
		}

		rel_path, err := uri.Id2RelPath(id, uri_args)

		if err != nil {
			return nil, err
		}

		fh, err := opts.Reader.Read(ctx, rel_path)

		if err != nil {
			return nil, err
		}

		defer fh.Close()

		source, err := io.ReadAll(fh)

		if err != nil {
			return nil, err
		}

		target, err := json.Marshal(r)

		if err != nil {
			return nil, err
		}

		target, err = AppendPropertiesWithJSON(ctx, opts, source, target)

		if err != nil {
			return nil, err
		}

		var props *PropertiesResponse
		err = json.Unmarshal(target, &props)

		if err != nil {
			return nil, err
		}

		new_results[idx] = props
	}

	props_rsp := &PropertiesResponseResults{
		Properties: new_results,
	}

	return props_rsp, nil
}

func AppendPropertiesWithJSON(ctx context.Context, opts *AppendPropertiesOptions, source []byte, target []byte) ([]byte, error) {

	var err error

	for _, e := range opts.Keys {

		paths := make([]string, 0)

		if strings.HasSuffix(e, "*") || strings.HasSuffix(e, ":") {

			e = strings.Replace(e, "*", "", -1)

			var props gjson.Result

			if opts.SourcePrefix != "" {
				props = gjson.GetBytes(source, opts.SourcePrefix)
			} else {
				props = gjson.ParseBytes(source)
			}

			for k, _ := range props.Map() {

				if strings.HasPrefix(k, e) {
					paths = append(paths, k)
				}
			}

		} else {
			paths = append(paths, e)
		}

		for _, p := range paths {

			get_path := p
			set_path := p

			if opts.SourcePrefix != "" {
				get_path = fmt.Sprintf("%s.%s", opts.SourcePrefix, get_path)
			}

			if opts.TargetPrefix != "" {
				set_path = fmt.Sprintf("%s.%s", opts.TargetPrefix, p)
			}

			v := gjson.GetBytes(source, get_path)

			/*
				log.Println("GET", get_path)
				log.Println("SET", set_path)
				log.Println("VALUE", v.Value())
			*/

			if !v.Exists() {
				continue
			}

			target, err = sjson.SetBytes(target, set_path, v.Value())

			if err != nil {
				return nil, err
			}
		}
	}

	return target, nil
}