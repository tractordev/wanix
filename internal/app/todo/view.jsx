
export default {
  view: ({attrs: {store}}) => {
    return (
      <main style="width: 512px;" class="h-full mx-auto mt-64 border-4 border-black bg-white p-4 flex flex-col gap-3">
        <h1 class="text-3xl italic uppercase">Todos</h1>

        <ul>
          {store.todos.map((t,idx) =>
            <li class="flex flex-row">
              <button onclick={() => store.completeTodo(idx, !t.completed)}>
                {m.trust(t.completed ? "[&check;]" : "[ ]")}
              </button>
              <div class={"flex-grow ml-2 " + (t.completed ? "line-through" : "")}>{t.title}</div>
              <button class="hover:text-red-500" 
                      onclick={() => store.removeTodo(idx)}>
                {"{x}"}
              </button>
            </li>)}
        </ul>

        <form class="flex flex-row" spellcheck="" autocapitalize="off" onsubmit={() => false}>
          [<input class="min-w-0 outline-none border-gray-200 border-dotted border-b-2 flex-grow"
                  type="text"
                  autofocus="true"
                  name="Title" />]
          <button class="hover:text-green-500" 
                  role="button" 
                  tabindex="-1"
                  onclick={() => {
                    let el = document.querySelector("[name=Title]");
                    store.addTodo(el.value);
                    el.value = "";
                  }}>
            {"{+}"}
          </button>
        </form>
      </main>
    )
  }
} 