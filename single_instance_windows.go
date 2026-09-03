package main

import "golang.org/x/sys/windows"

// "Global\" 접두사로 세션 전체(다른 사용자 세션에서 뜬 것 포함) 기준으로 중복 실행을 잡는다.
const singleInstanceMutexName = `Global\moonkata-sync-server-single-instance`

// acquireSingleInstanceLock은 이미 이 프로그램이 실행 중이면 false를 돌려준다. 이름 있는 뮤텍스로
// 판정하는 표준 Windows 단일 인스턴스 패턴 — 파일 락을 직접 구현할 필요 없이, 뮤텍스를 이미 다른
// 프로세스가 갖고 있는지를 OS가 원자적으로 판정해준다. 프로세스가 어떻게 죽든(정상 종료, 강제 종료,
// 크래시) OS가 알아서 뮤텍스를 정리하므로 우리가 따로 잠금 해제를 신경 쓸 필요가 없다.
func acquireSingleInstanceLock() bool {
	name, err := windows.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		return true // 이름 자체를 못 만드는 경우(사실상 없음) — 중복 검사를 포기하고 그냥 진행
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		return true // 뮤텍스 생성 자체가 실패 — 중복 검사를 포기하고 그냥 진행(막지 않는 쪽이 안전)
	}
	// CreateMutex는 이미 존재하는 뮤텍스를 열었을 때도 유효한 핸들을 돌려주면서 err를
	// ERROR_ALREADY_EXISTS로 채운다(Win32 GetLastError 관례) — 그래서 핸들 유효성과 이 에러 코드를
	// 따로 확인해야 한다.
	return err != windows.ERROR_ALREADY_EXISTS
}
