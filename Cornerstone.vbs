' Cornerstone 启动脚本 - 无窗口模式
' 双击运行，后台启动服务并自动打开浏览器

Option Explicit

Dim objShell, objFSO, strPath, objWMIService, colProcesses
Dim bRunning, iPortCheck, objHTTP

Set objShell = CreateObject("WScript.Shell")
Set objFSO = CreateObject("Scripting.FileSystemObject")

' 获取程序所在目录
strPath = objFSO.GetParentFolderName(WScript.ScriptFullName)
objShell.CurrentDirectory = strPath

' 检查程序是否存在
If Not objFSO.FileExists(strPath & "\cornerstone.exe") Then
    MsgBox "未找到 cornerstone.exe，请确保程序文件完整。", vbCritical, "Cornerstone"
    WScript.Quit 1
End If

' 检查是否已在运行
Set objWMIService = GetObject("winmgmts:\\.\root\cimv2")
Set colProcesses = objWMIService.ExecQuery("Select * from Win32_Process Where Name = 'cornerstone.exe'")

bRunning = False
For Each objProcess in colProcesses
    bRunning = True
    Exit For
Next

If bRunning Then
    ' 如果已在运行，直接打开浏览器
    objShell.Run "http://localhost:8080", 1, False
    MsgBox "Cornerstone 已在运行。" & vbCrLf & vbCrLf & "访问地址: http://localhost:8080", vbInformation, "Cornerstone"
Else
    ' 启动服务
    objShell.Run "cornerstone.exe", 0, False

    ' 等待服务启动
    WScript.Sleep 3000

    ' 检查服务是否成功启动
    iPortCheck = CheckPort()

    If iPortCheck = 0 Then
        ' 成功启动
        objShell.Run "http://localhost:8080", 1, False
        MsgBox "Cornerstone 启动成功！" & vbCrLf & vbCrLf & _
               "访问地址: http://localhost:8080" & vbCrLf & vbCrLf & _
               "提示: 关闭此窗口不会影响服务运行。" & vbCrLf & _
               "如需停止服务，请打开任务管理器结束 cornerstone.exe", _
               vbInformation, "Cornerstone"
    Else
        ' 启动可能较慢，仍然打开浏览器
        objShell.Run "http://localhost:8080", 1, False
        MsgBox "Cornerstone 正在启动中，请稍等..." & vbCrLf & vbCrLf & _
               "如果页面无法访问，请稍后再试。", vbExclamation, "Cornerstone"
    End If
End If

' 清理
Set objShell = Nothing
Set objFSO = Nothing
Set objWMIService = Nothing
Set colProcesses = Nothing

WScript.Quit 0

' 检查端口是否可用
Function CheckPort()
    On Error Resume Next
    Set objHTTP = CreateObject("Microsoft.XMLHTTP")
    objHTTP.open "GET", "http://localhost:8080/health", False
    objHTTP.send
    If Err.Number = 0 And objHTTP.Status = 200 Then
        CheckPort = 0
    Else
        CheckPort = 1
    End If
    On Error GoTo 0
    Set objHTTP = Nothing
End Function
