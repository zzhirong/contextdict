function* fun(){
    yield 1
    yield 2
    yield 3
}

let f = fun()
for(v of f){
    print(v)
}
