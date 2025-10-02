package main

import (
	"KernelFactory/builder"
	"KernelFactory/downloader"
	"fmt"
)

func main() {
	fmt.Println("Starting kernel factory...")
	KernelOut := downloader.DownloadKernel("6.17")
	if KernelOut == nil {
		fmt.Println("Failed to download kernel.")
		return
	}
	builder.BuildKernel(KernelOut, "6.17", "./output")
	NvidiaOut := downloader.DownloadNvidiaDriver("580.82.09")
	if NvidiaOut == nil {
		fmt.Println("Failed to download NVIDIA driver.")
		return
	}
	builder.BuildNvidiaDriver(NvidiaOut, "580.82.09", "./output")
}
