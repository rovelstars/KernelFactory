package utils

import "os/exec"
import "fmt"
import "os"

func ExtractTar(src, dst string) error {
  if _, err := os.Stat(dst); os.IsNotExist(err) {
    err := os.MkdirAll(dst, 0755)
    if err != nil {
      return fmt.Errorf("failed to create destination directory: %v", err)
    }
  }
  cmd := exec.Command("tar", "-xf", src, "-C", dst)
  err := cmd.Run()
  if err != nil {
    return fmt.Errorf("failed to extract tar file: %v", err)
  }
  return nil
}

func MakeWithAllCores(src, arg string, envs ...string) {
    fmt.Printf("Making with all cores in %s\n", src)
    // just run make -j$(nproc) in src
    cmd := exec.Command("bash", "-lc", fmt.Sprintf("make %s -j$(nproc)", arg))
    if len(envs) > 0 {
      cmd.Env = append(os.Environ(), envs...)
    }
    cmd.Dir = src
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Run()
    if err != nil {
        fmt.Println("Error making:", err)
        return
    }
    fmt.Println("Make completed successfully")
}