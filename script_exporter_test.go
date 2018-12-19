package main

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
	"github.com/stretchr/testify/assert"
	"fmt"
)

var config = &Config{
	Scripts: []*Script{
		{"success", "exit 0", 1},
		{"failure", "exit 1", 1},
		{"timeout", "sleep 5", 2},
	},
}

func TestCombineYamlScripts(t *testing.T) {
	fileContent1 := []byte(`
scripts:
 - name: mesos_slave_process_check
   script: ps waux | grep -q mesos.slave
 
 - name: ssh_port_check
   script: nc -z -w10 $(hostname -i) 22`)

	fileContent2 := []byte(`
scripts:
 - name: apache_process_check
   script: nc -z -w10 localhost 80

 - name: java_process_check
   script: nc -z -w10 localhost 8080`)

	tmpFile1, err := ioutil.TempFile("", "testFile1")
	tmpFile2, err := ioutil.TempFile("", "testFile2")

	tmpFile1.Write(fileContent1)
	tmpFile2.Write(fileContent2)

	defer os.Remove(tmpFile1.Name())
	defer os.Remove(tmpFile2.Name())

	fileNames := []string{tmpFile1.Name(), tmpFile2.Name()}
	combinedConfigs, err := combineYamlScripts(fileNames)

	if err != nil {
		log.Fatal(err)
	}

	expectedResult := `scripts:
- name: mesos_slave_process_check
  script: ps waux | grep -q mesos.slave
  timeout: 0
- name: ssh_port_check
  script: nc -z -w10 $(hostname -i) 22
  timeout: 0
- name: apache_process_check
  script: nc -z -w10 localhost 80
  timeout: 0
- name: java_process_check
  script: nc -z -w10 localhost 8080
  timeout: 0
`
   assert.Equal(t, string(combinedConfigs), expectedResult)
}

func TestReadYamlsInDirOrFile1(t *testing.T) { // For Directory

	fileContent1 := []byte(`
scripts:
 - name: mesos_slave_process_check
   script: ps waux | grep -q mesos.slave
 
 - name: ssh_port_check
   script: nc -z -w10 $(hostname -i) 22`)

	fileContent2 := []byte(`
scripts:
 - name: apache_process_check
   script: nc -z -w10 localhost 80

 - name: java_process_check
   script: nc -z -w10 localhost 8080`)

	dir, err := ioutil.TempDir("/tmp","script_exporter")
	tmpFile1, err := ioutil.TempFile(dir, "testFile1")
	tmpFile2, err := ioutil.TempFile(dir, "testFile2")

	tmpFile1.Write(fileContent1)
	tmpFile2.Write(fileContent2)

	defer os.Remove(tmpFile1.Name())
	defer os.Remove(tmpFile2.Name())
	defer os.Remove(dir)

   	finalContent, err := readYamlsinDirOrFile(dir)

   	if err != nil {
   		fmt.Println("Error: ", err)
   	}

	expectedResult := `scripts:
- name: mesos_slave_process_check
  script: ps waux | grep -q mesos.slave
  timeout: 0
- name: ssh_port_check
  script: nc -z -w10 $(hostname -i) 22
  timeout: 0
- name: apache_process_check
  script: nc -z -w10 localhost 80
  timeout: 0
- name: java_process_check
  script: nc -z -w10 localhost 8080
  timeout: 0
`
   assert.Equal(t, string(finalContent), expectedResult)	
}

func TestReadYamlsInDirOrFile2(t *testing.T) { // For 

	fileContent1 := []byte(`
scripts:
 - name: mesos_slave_process_check
   script: ps waux | grep -q mesos.slave
 
 - name: ssh_port_check
   script: nc -z -w10 $(hostname -i) 22`)

	tmpFile1, err := ioutil.TempFile("", "testFile1")
	tmpFile1.Write(fileContent1)
	defer os.Remove(tmpFile1.Name())

	finalContent, err := readYamlsinDirOrFile(tmpFile1.Name())

	if err != nil {
		fmt.Println("Error: ", err)
	}

	expectedResult := `scripts:
- name: mesos_slave_process_check
  script: ps waux | grep -q mesos.slave
  timeout: 0
- name: ssh_port_check
  script: nc -z -w10 $(hostname -i) 22
  timeout: 0
`
   assert.Equal(t, string(finalContent), expectedResult)	
}

func TestRunScripts(t *testing.T) {
	measurements := runScripts(config.Scripts)

	expectedResults := map[string]struct {
		success     int
		minDuration float64
	}{
		"success": {1, 0},
		"failure": {0, 0},
		"timeout": {0, 2},
	}

	for _, measurement := range measurements {
		expectedResult := expectedResults[measurement.Script.Name]

		if measurement.Success != expectedResult.success {
			t.Errorf("Expected result not found: %s", measurement.Script.Name)
		}

		if measurement.Duration < expectedResult.minDuration {
			t.Errorf("Expected duration %f < %f: %s", measurement.Duration, expectedResult.minDuration, measurement.Script.Name)
		}
	}
}

func TestScriptFilter(t *testing.T) {
	t.Run("RequiredParameters", func(t *testing.T) {
		_, err := scriptFilter(config.Scripts, "", "")

		if err.Error() != "`name` or `pattern` required" {
			t.Errorf("Expected failure when supplying no parameters")
		}
	})

	t.Run("NameMatch", func(t *testing.T) {
		scripts, err := scriptFilter(config.Scripts, "success", "")

		if err != nil {
			t.Errorf("Unexpected: %s", err.Error())
		}

		if len(scripts) != 1 || scripts[0] != config.Scripts[0] {
			t.Errorf("Expected script not found")
		}
	})

	t.Run("PatternMatch", func(t *testing.T) {
		scripts, err := scriptFilter(config.Scripts, "", "fail.*")

		if err != nil {
			t.Errorf("Unexpected: %s", err.Error())
		}

		if len(scripts) != 1 || scripts[0] != config.Scripts[1] {
			t.Errorf("Expected script not found")
		}
	})

	t.Run("AllMatch", func(t *testing.T) {
		scripts, err := scriptFilter(config.Scripts, "success", ".*")

		if err != nil {
			t.Errorf("Unexpected: %s", err.Error())
		}

		if len(scripts) != 3 {
			t.Fatalf("Expected 3 scripts, received %d", len(scripts))
		}

		for i, script := range config.Scripts {
			if scripts[i] != script {
				t.Fatalf("Expected script not found")
			}
		}
	})
}
