package downloader

import (
  "fmt"
  "io"
  "net/http"
  "os"
)

func DownloadKernel(version string) *os.File {
  url := fmt.Sprintf("https://cdn.kernel.org/pub/linux/kernel/v%s.x/linux-%s.tar.xz", version[:1], version)
  //check if linux-version.tar.xz already exists
  if _, err := os.Stat(fmt.Sprintf("linux-%s.tar.xz", version)); err == nil {
    fmt.Println("Kernel version", version, "already downloaded.")
    out, err := os.Open(fmt.Sprintf("linux-%s.tar.xz", version))
    if err != nil {
      fmt.Println("Error opening file:", err)
      return nil
    }
    return out
  }
  fmt.Println("Downloading kernel from:", url)
  resp, err := http.Get(url)
  if err != nil {
    fmt.Println("Error downloading kernel:", err)
    return nil
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    fmt.Println("Failed to download kernel, status code:", resp.StatusCode)
    return nil
  }
  out, err := os.Create(fmt.Sprintf("linux-%s.tar.xz", version))
  if err != nil {
    fmt.Println("Error creating file:", err)
    return nil
  }
  defer out.Close()

  _, err = io.Copy(out, resp.Body)
  if err != nil {
    fmt.Println("Error saving file:", err)
    return nil
  }
  fmt.Println("Kernel downloaded successfully:", out.Name())
  return out
}

func DownloadNvidiaDriver(version string) *os.File {
    //https://github.com/NVIDIA/open-gpu-kernel-modules/archive/refs/tags/580.82.09.tar.gz
    url := fmt.Sprintf("https://github.com/NVIDIA/open-gpu-kernel-modules/archive/refs/tags/%s.tar.gz", version)
    //check if nvidia-driver-version.tar.gz already exists
    if _, err := os.Stat(fmt.Sprintf("nvidia-driver-%s.tar.gz", version)); err == nil {
        fmt.Println("NVIDIA driver version", version, "already downloaded.")
        out, err := os.Open(fmt.Sprintf("nvidia-driver-%s.tar.gz", version))
        if err != nil {
            fmt.Println("Error opening file:", err)
            return nil
        }
        return out
    }
    fmt.Println("Downloading NVIDIA driver from:", url)
    resp, err := http.Get(url)
    if err != nil {
        fmt.Println("Error downloading NVIDIA driver:", err)
        return nil
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        fmt.Println("Failed to download NVIDIA driver, status code:", resp.StatusCode)
        return nil
    }
    out, err := os.Create(fmt.Sprintf("nvidia-driver-%s.tar.gz", version))
    if err != nil {
        fmt.Println("Error creating file:", err)
        return nil
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        fmt.Println("Error saving file:", err)
        return nil
    }
    fmt.Println("NVIDIA driver downloaded successfully:", out.Name())
    return out
}
