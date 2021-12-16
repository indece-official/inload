package assets

import "io/ioutil"

// ReadFile reads a asseet file as string
func ReadFile(filename string) (string, error) {
	file, err := Assets.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
