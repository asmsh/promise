// Package status represents values for the promise's status.
//
// The value is split into 5 sections, flags, state, fate, chain, and lock, as
// follows, starting from the right:
// - The lock section takes 4 bits.
// - The chain section takes 6 bits.
// - The fate section takes 2 bits.
// - The state section takes 2 bits.
// - The flags section takes 8 bits.
//
// Description of the section:
//
//   - the lock section.
//     = although it's named 'lock', it doesn't use any Mutexes.
//     = The lock is implemented through atomic writing, reading, and updating of
//     the status value.
//     = although the implementation is lock-free(in the sense of Mutexes), it's
//     not wait-free.
//     = The lock logic used is just a way to tell new comers(that want to update
//     the status) that: "the value is currently being updated by some previous
//     update call so wait here until it finish, then you can get your chance
//     to update the status too".
//     = Currently, for a large number of concurrent calls, the lock logic might
//     starve some calls which are waiting to acquire the lock.
//     = Currently, there's no fairness in the lock logic, an old call waiting to
//     acquire the lock might wait longer than newer calls.
//     = The whole waiting behaviour is passed to the 'go scheduler'(through a call
//     to runtime.Gosched) to decide which goroutine should run now(and hence
//     acquire the lock first).
//     = Despite the fact of the lock fairness problem, the lock will be acquired
//     for only a small period of time by any call, because the operations done
//     while the lock is acquired are very basic(add, and, or, assign, compare,
//     and conditions).
//     = Based on the expected small acquire time, when no concurrent acquire calls
//     are executed continuously all the time, no starving acquire calls will run
//     forever.
//     = From testing(and benchmarks), the current lock implementation does just
//     okay, it does the job, without being a bottleneck.
//     = Considering the starving issue, it's rather odd to have such big number
//     of concurrent calls on a promise in real examples, so the issue should
//     not be visible at all.
//
//   - The chain section describes the promise chain and the chained calls.
//     = 3 mutually exclusive possible modes, represented by 2 bits:
//
//   - none: no calls are chained or has ever been chained to this promise.
//
//   - wait: the chain has a call that can read only fulfilled promise results,
//
//   - read: the chain has a call that can read fulfilled and rejected promise
//     results, such call can prevent UnCaughtErr panics if the promise
//     is about to be rejected.
//
//   - Follow: the chain has a call that can read all types of promise results,
//     such call can prevent panics if the promise is about to be
//     rejected or panicked.
//     = 1 flag(with 3 more reserved):
//
//   - calledFinally: whether a 'Finally' callback has been called on this
//     promise or not.
//     = Wait calls are 'Wait', 'WaitUntil 'Res', and 'GetResUntil'.
//     = Read calls are 'Finally', and 'asyncRead'.
//     = Follow calls are 'Then', 'Catch', 'Recover', and 'asyncFollow'.
//     = The chain modes decides whether a panic should happen when the promise
//     is about to be rejected or panicked(uncaught error or un-recovered panic).
//     = Not all methods have to updated the chain mode with their corresponding
//     chain mode value, only the ones which their logic depends on the panic
//     behaviour.
//     = The chain value is written/updated as long as the fate is unresolved.
//     = The chain value is read just before the promise is resolved.
//     = Each chain value covers the logic of the previous values.
//     = The 'wait' chain mode is defined but not used, as currently, no specific
//     logic needs to know about it.
//
//   - The fate section describes the fate of the promise.
//     = 4 mutually exclusive possible values, represented by 2 bits:
//
//   - unresolved: the promise's callback func didn't finish its work, or its
//     wait chan hasn't been closed nor received the result.
//
//   - Resolving: the promise's result is being passed to the only one resolving
//     call.
//     It's an internal fate that denotes that other calls must wait.
//     It's considered an equivalent to the unresolved fate.
//
//   - Resolved: the promise's final state is know, whether it's still pending,
//     or is settled to some other state.
//
//   - Handled: the promise is resolved and its result is being passed to a
//     read call, or to any follow call.
//     = The fate value is updated twice, firstly, when resolving, along with the
//     state value(after reading the chain value), and secondly, when executing
//     a follow method's callback or returning from an await call.
//     = A promise which its fate is unresolved, its state must be pending.
//     = A promise which its fate is resolved, and its state is pending, its fate
//     will never be handled.
//     = At transiting from the unresolved fate to the resolved fate, the chain
//     value is read and a panic will occur if the promise is not followed,
//     and its state is panicked or rejected(when running in the safe mode).
//
//   - The state section describes the state of this promise.
//     = 4 mutually exclusive possible values, represented by 2 bits:
//
//   - pending: the promise's callback func didn't finish its work, or its
//     wait chan hasn't been closed nor received the result, yet.
//
//   - Fulfilled: the promise's callback func finished its work and returned
//     the result, the wait chan received the result, or the wait
//     chan has been closed.
//     In any case, where there's a result, the last value must be
//     either not an error value, or it's a nil error value.
//
//   - Rejected: the promise's callback func finished its work and returned
//     the result, or the wait chan received the result.
//     In any case, the last value must be a non-nil error value.
//
//   - Panicked: the promise's callback func produced a panic which it didn't
//     recover from.
//     = The state value is written once, after reading the chain value.
//     = A promise which its state is pending, its fate must be either unresolved
//     or resolved.
//     = A promise which its state is fulfilled, rejected, or panicked, its fate
//     must be either resolved or handled.
//
//   - The flags section describes the behaviour of the promise, it consists of
//     two sub-sections, types, and features.
//     = The types sub-section holds info about the type of the promise, and has
//     two values(with 2 more reserved):
//
//   - once: whether the promise is a OncePromise or not.
//
//   - Timed: whether the promise is a TimedPromise or not.
//     = The features sub-section holds info about supported features by the promise,
//     and has two values(with 2 more reserved):
//
//   - notSafe: whether the promise is running in the non-safe mode or not.
//
//   - External: whether the wait chan is created externally or internally.
//     = The flags are written once, at creation, and never updated.
package status
